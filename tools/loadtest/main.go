// Alat uji beban Modul 9.
//
// CATATAN: modul menyebut file ini "dibagikan dosen". Bila Anda memiliki
// versi resmi dari dosen, pakai versi itu. Versi ini ditulis mengikuti
// potongan kode dan contoh keluaran pada modul (fungsi fire, mode crowd,
// credits, swap) agar uji pada Bagian C dapat dijalankan.
//
// Pemakaian (server harus sedang berjalan di terminal lain):
//
//	go run ./tools/loadtest -mode crowd
//	go run ./tools/loadtest -mode credits
//	go run ./tools/loadtest -mode swap
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"latihan-fiber/app/model"
	"latihan-fiber/config"
	"latihan-fiber/database"
	"latihan-fiber/helper"
)

var (
	baseURL string
	client  = &http.Client{Timeout: 15 * time.Second}
)

func main() {
	mode := flag.String("mode", "crowd", "crowd | credits | swap")
	flag.StringVar(&baseURL, "base", "http://localhost:3000/api/v1", "alamat API")
	flag.Parse()

	config.LoadEnv()
	ctx := context.Background()

	pool, err := database.NewPool(ctx)
	if err != nil {
		fail("database: %v", err)
	}
	defer pool.Close()

	jwtManager := helper.NewJWTManager(
		config.GetEnv("JWT_SECRET", ""),
		config.GetEnv("JWT_ISSUER", "praktikum-backend"),
		time.Duration(config.GetEnvInt("JWT_ACCESS_TTL_MINUTES", 15))*time.Minute,
	)

	tokens, err := loadTokens(ctx, pool, jwtManager)
	if err != nil {
		fail("menyiapkan token: %v", err)
	}
	courses, err := loadCourses(ctx, pool)
	if err != nil {
		fail("membaca kelas: %v", err)
	}

	switch *mode {
	case "crowd":
		runCrowd(ctx, pool, tokens, courses)
	case "credits":
		runCredits(ctx, pool, tokens, courses)
	case "swap":
		runSwap(ctx, pool, tokens, courses)
	default:
		fail("mode tidak dikenal: %s", *mode)
	}
}

// fire menjalankan n pekerjaan secara serentak dan menghitung hasilnya.
//
// Setiap goroutine menunggu di depan "gerbang" start yang sama. Gerbang
// baru dibuka setelah SELURUH goroutine siap, sehingga request benar-benar
// berangkat hampir bersamaan — bukan bergiliran sesuai urutan dibuatnya.
func fire(n int, job func(i int) string) map[string]int {
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan string, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int, out chan<- string) {
			defer wg.Done()
			<-start
			out <- job(i)
		}(i, results)
	}

	close(start) // buka gerbang
	wg.Wait()    // tunggu SEMUA goroutine selesai
	close(results)

	tally := map[string]int{}
	for r := range results {
		tally[r]++
	}
	return tally
}

// ---------------------------------------------------------------- mode

func runCrowd(ctx context.Context, pool *pgxpool.Pool, tokens []string, courses map[string]int) {
	fmt.Println("60 mahasiswa berebut kelas IF399 secara serentak...")
	id := courses["IF399"]

	began := time.Now()
	tally := fire(60, func(i int) string {
		return call(http.MethodPost, fmt.Sprintf("/courses/%d/enroll", id), tokens[i], nil)
	})
	printTally(tally, time.Since(began))
	checkCourse(ctx, pool, "IF399")
}

func runCredits(ctx context.Context, pool *pgxpool.Pool, tokens []string, courses map[string]int) {
	codes := []string{"IF101", "IF102", "IF301", "IF302", "IF303", "IF304",
		"IF305", "IF306", "IF307", "IF308", "IF309", "IF310"}
	fmt.Println("10 mahasiswa, masing-masing mendaftar ke 12 kelas 3 SKS secara serentak (batas 24 SKS)...")

	began := time.Now()
	tally := fire(10*len(codes), func(i int) string {
		student := i / len(codes)
		code := codes[i%len(codes)]
		return call(http.MethodPost, fmt.Sprintf("/courses/%d/enroll", courses[code]), tokens[student], nil)
	})
	printTally(tally, time.Since(began))

	over := 0
	for s := 1; s <= 10; s++ {
		var total int
		_ = pool.QueryRow(ctx,
			`SELECT COALESCE(SUM(c.credits),0) FROM enrollments e
			 JOIN courses c ON c.id = e.course_id
			 JOIN users u ON u.id = e.user_id
			 WHERE u.username = $1`, fmt.Sprintf("mhs%03d", s)).Scan(&total)
		if total > 24 {
			over++
			fmt.Printf(" mhs%03d mengambil %d SKS  -> MELEBIHI BATAS\n", s, total)
		}
	}
	if over == 0 {
		fmt.Println(" seluruh 10 mahasiswa <= 24 SKS -> KONSISTEN")
	} else {
		fmt.Printf(" %d mahasiswa melebihi 24 SKS -> TIDAK KONSISTEN\n", over)
	}
	for _, code := range codes {
		checkCourseQuiet(ctx, pool, code)
	}
}

func runSwap(ctx context.Context, pool *pgxpool.Pool, tokens []string, courses map[string]int) {
	a, b := courses["IF101"], courses["IF102"]

	// Persiapan (bergiliran): mhs001-020 di IF101, mhs021-040 di IF102.
	for i := 0; i < 40; i++ {
		target := a
		if i >= 20 {
			target = b
		}
		if r := call(http.MethodPost, fmt.Sprintf("/courses/%d/enroll", target), tokens[i], nil); !strings.HasPrefix(r, "201") {
			fail("persiapan mode swap gagal untuk mhs%03d: %s (jalankan reset.sql dulu)", i+1, r)
		}
	}

	var deadlocksBefore int64
	_ = pool.QueryRow(ctx, "SELECT deadlocks FROM pg_stat_database WHERE datname = current_database()").Scan(&deadlocksBefore)

	fmt.Println("20 pasang mahasiswa pindah kelas berlawanan arah secara serentak...")
	began := time.Now()
	tally := fire(40, func(i int) string {
		from, to := a, b
		if i >= 20 {
			from, to = b, a
		}
		body := map[string]int{"from_course_id": from, "to_course_id": to}
		return call(http.MethodPost, "/enrollments/swap", tokens[i], body)
	})
	printTally(tally, time.Since(began))

	var deadlocksAfter int64
	_ = pool.QueryRow(ctx, "SELECT deadlocks FROM pg_stat_database WHERE datname = current_database()").Scan(&deadlocksAfter)
	fmt.Printf(" deadlock terdeteksi PostgreSQL selama uji: %d\n", deadlocksAfter-deadlocksBefore)
	checkCourse(ctx, pool, "IF101")
	checkCourse(ctx, pool, "IF102")
}

// -------------------------------------------------------------- helper

// call mengirim satu request dan mengembalikan "STATUS KODE".
func call(method, path, token string, body any) string {
	var reader io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = strings.NewReader(string(raw))
	}
	req, _ := http.NewRequest(method, baseURL+path, reader)
	req.Header.Set("Authorization", "Bearer "+token)
	// Route /courses dijaga RequireJSON: POST tanpa Content-Type JSON
	// ditolak 415 walaupun body-nya kosong.
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "ERR " + err.Error()
	}
	defer resp.Body.Close()

	var parsed struct {
		Code string `json:"code"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&parsed)

	switch {
	case resp.StatusCode < 300:
		return fmt.Sprintf("%d OK", resp.StatusCode)
	case parsed.Code != "":
		return fmt.Sprintf("%d %s", resp.StatusCode, parsed.Code)
	default:
		return fmt.Sprintf("%d", resp.StatusCode)
	}
}

func printTally(tally map[string]int, took time.Duration) {
	keys := make([]string, 0, len(tally))
	for k := range tally {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	fmt.Println(" hasil request:")
	for _, k := range keys {
		fmt.Printf("   %-30s %d\n", k, tally[k])
	}
	fmt.Printf(" waktu total : %dms\n", took.Milliseconds())
}

func checkCourse(ctx context.Context, pool *pgxpool.Pool, code string) {
	var quota, count, rows int
	_ = pool.QueryRow(ctx,
		`SELECT c.quota, c.enrolled_count,
		        (SELECT COUNT(*) FROM enrollments e WHERE e.course_id = c.id)
		 FROM courses c WHERE c.code = $1`, code).Scan(&quota, &count, &rows)

	status := "KONSISTEN"
	if count != rows || rows > quota {
		status = "TIDAK KONSISTEN"
	}
	fmt.Printf(" %s kuota=%d enrolled_count=%d baris enrollments=%d -> %s\n",
		code, quota, count, rows, status)
}

func checkCourseQuiet(ctx context.Context, pool *pgxpool.Pool, code string) {
	var count, rows int
	_ = pool.QueryRow(ctx,
		`SELECT c.enrolled_count, (SELECT COUNT(*) FROM enrollments e WHERE e.course_id = c.id)
		 FROM courses c WHERE c.code = $1`, code).Scan(&count, &rows)
	if count != rows {
		fmt.Printf(" %s enrolled_count=%d baris enrollments=%d -> TIDAK KONSISTEN\n", code, count, rows)
	}
}

// loadTokens membuat access token untuk mhs001 sampai mhs060.
//
// Alat ini mencetak token sendiri karena ia memegang JWT_SECRET.
// Itulah sebabnya JWT_SECRET tidak boleh bocor: siapa pun yang
// memegangnya dapat mencetak token atas nama siapa saja.
func loadTokens(ctx context.Context, pool *pgxpool.Pool, jwtManager *helper.JWTManager) ([]string, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, username, role FROM users
		 WHERE username ~ '^mhs[0-9]{3}$' ORDER BY username LIMIT 60`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tokens := []string{}
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.Username, &u.Role); err != nil {
			return nil, err
		}
		// PERBAIKAN M9-#3: potongan kode di modul memanggil
		// jwtManager.GenerateAccessToken(u), padahal method JWTManager
		// (Modul 5) bernama GenerateAccess. Karena "go build ./..." ikut
		// mengompilasi tools/loadtest, kesalahan ini ditolak compiler.
		token, err := jwtManager.GenerateAccess(u)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}
	if len(tokens) < 60 {
		return nil, fmt.Errorf("hanya %d akun mhs ditemukan; jalankan migrations/006_seed_loadtest.sql", len(tokens))
	}
	return tokens, rows.Err()
}

func loadCourses(ctx context.Context, pool *pgxpool.Pool) (map[string]int, error) {
	rows, err := pool.Query(ctx, "SELECT code, id FROM courses")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[string]int{}
	for rows.Next() {
		var code string
		var id int
		if err := rows.Scan(&code, &id); err != nil {
			return nil, err
		}
		result[code] = id
	}
	return result, rows.Err()
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "loadtest: "+format+"\n", args...)
	os.Exit(1)
}