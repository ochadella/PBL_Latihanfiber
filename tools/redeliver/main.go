// Command redeliver menyalin satu event outbox menjadi beberapa task
// tambahan di Redis, tanpa mengubah tabel outbox.
//
// Dipakai untuk membuktikan idempotensi worker: beberapa salinan task yang
// sama harus tetap menghasilkan satu notifikasi dan satu email.
//
// Contoh:
//
//	go run ./tools/redeliver -outbox 42 -copies 5
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/hibiken/asynq"

	"latihan-fiber/app/model"
	"latihan-fiber/app/service"
	"latihan-fiber/config"
	"latihan-fiber/database"
)

func main() {
	outboxID := flag.Int64("outbox", 0, "id baris outbox yang akan diduplikasi (wajib)")
	copies := flag.Int("copies", 1, "jumlah salinan task yang dibuat")
	flag.Parse()

	if *outboxID <= 0 {
		fmt.Fprintln(os.Stderr, "kesalahan: -outbox wajib diisi dengan id positif")
		flag.Usage()
		os.Exit(1)
	}
	if *copies <= 0 {
		fmt.Fprintln(os.Stderr, "kesalahan: -copies harus lebih besar dari 0")
		os.Exit(1)
	}

	config.LoadEnv()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := database.NewPool(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gagal terhubung ke database: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	var event model.OutboxEvent
	err = pool.QueryRow(ctx, `
		SELECT id, event_type, payload, created_at
		FROM outbox
		WHERE id = $1`, *outboxID).Scan(
		&event.ID, &event.EventType, &event.Payload, &event.CreatedAt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gagal membaca outbox %d: %v\n", *outboxID, err)
		os.Exit(1)
	}

	// Event perubahan kuota menghasilkan satu job per peserta. Redeliver
	// harus menyalin SEMUA job itu; menyalin satu job saja tidak menguji
	// idempotensi untuk penerima lainnya.
	jobs, err := service.JobsForEvent(event)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gagal memecah event: %v\n", err)
		os.Exit(1)
	}

	client := asynq.NewClient(config.RedisOpt())
	defer client.Close()

	// TaskID harus BERBEDA untuk setiap salinan; memakai id outbox yang sama
	// akan ditolak Redis sebagai konflik dan hanya satu task yang masuk.
	nonce := time.Now().UnixNano()
	enqueued := 0
	for i := 0; i < *copies; i++ {
		for _, job := range jobs {
			payload, err := json.Marshal(job)
			if err != nil {
				fmt.Fprintf(os.Stderr, "gagal menyusun payload: %v\n", err)
				os.Exit(1)
			}
			task := asynq.NewTask(model.TaskSendNotification, payload,
				asynq.TaskID(fmt.Sprintf("redeliver-%d-%d-%d-u%d",
					*outboxID, nonce, i, job.UserID)),
				asynq.Queue(model.QueueNotifications),
				asynq.MaxRetry(5),
				asynq.Timeout(30*time.Second),
				asynq.Retention(24*time.Hour),
			)
			if _, err := client.EnqueueContext(ctx, task); err != nil {
				fmt.Fprintf(os.Stderr, "gagal enqueue salinan ke-%d: %v\n", i+1, err)
				os.Exit(1)
			}
			enqueued++
		}
	}

	fmt.Printf("%d salinan x %d penerima = %d task untuk outbox %d (%s) berhasil di-enqueue\n",
		*copies, len(jobs), enqueued, *outboxID, event.EventType)
}
