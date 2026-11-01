package model

import (
	"encoding/json"
	"time"
)

// Nama-nama event yang dicatat ke outbox. Ditulis sebagai konstanta,
// bukan string yang diketik ulang di banyak tempat, karena salah ketik pada
// nama event tidak ditangkap compiler maupun database.
const (
	EventEnrollmentCreated  = "enrollment.created"
	EventEnrollmentDropped  = "enrollment.dropped"
	EventEnrollmentSwapped  = "enrollment.swapped"
	EventCourseQuotaChanged = "course.quota_changed"
)

// TaskSendNotification adalah nama JENIS task (task type); worker memakainya untuk
// memilih handler. QueueNotifications adalah nama QUEUE tempat task
// itu menunggu; worker hanya mengambil task dari queue yang ia dengarkan.
const (
	TaskSendNotification = "notification:send"
	QueueNotifications   = "notifications"
)

// EnrollmentEvent adalah isi sebuah event KRS.
// Nama dan kode kelas DISALIN ke dalam event, bukan hanya id-nya.
type EnrollmentEvent struct {
	UserID         int    `json:"user_id"`
	CourseID       int    `json:"course_id"`
	CourseCode     string `json:"course_code"`
	CourseName     string `json:"course_name"`
	FromCourseCode string `json:"from_course_code,omitempty"`
	FromCourseName string `json:"from_course_name,omitempty"`
}

// QuotaChangedEvent adalah isi event perubahan kuota.
//
// Recipients adalah SNAPSHOT peserta pada saat kuota diubah, bukan hasil
// query ulang saat worker mengirim. Dengan begitu mahasiswa yang mendaftar
// setelah perubahan tidak menerima email untuk perubahan yang terjadi
// sebelum ia bergabung, dan mahasiswa yang keluar sesudahnya tetap
// menerima email untuk perubahan yang terjadi saat ia masih peserta.
//
// Satu event ini menghasilkan BANYAK notifikasi, satu per penerima.
type QuotaChangedEvent struct {
	CourseID   int    `json:"course_id"`
	CourseCode string `json:"course_code"`
	CourseName string `json:"course_name"`
	OldQuota   int    `json:"old_quota"`
	NewQuota   int    `json:"new_quota"`
	Recipients []int  `json:"recipients"`
}

// NotificationEvent adalah gabungan field yang dibutuhkan worker untuk
// menyusun pesan. Payload event KRS maupun perubahan kuota dapat
// di-unmarshal ke struct ini; field yang tidak relevan dibiarkan nol.
type NotificationEvent struct {
	UserID         int    `json:"user_id,omitempty"`
	CourseID       int    `json:"course_id"`
	CourseCode     string `json:"course_code"`
	CourseName     string `json:"course_name"`
	FromCourseCode string `json:"from_course_code,omitempty"`
	FromCourseName string `json:"from_course_name,omitempty"`
	OldQuota       int    `json:"old_quota,omitempty"`
	NewQuota       int    `json:"new_quota,omitempty"`
}

// OutboxEvent adalah satu baris tabel outbox.
type OutboxEvent struct {
	ID        int64
	EventType string
	Payload   []byte
	CreatedAt time.Time
}

// NotificationJob adalah isi task yang dikirim ke queue.
//
// OutboxID + UserID adalah idempotency key: satu event outbox boleh
// menghasilkan banyak task, satu untuk setiap penerima, tetapi setiap
// pasangan (outbox_id, user_id) hanya boleh mengirim satu email.
type NotificationJob struct {
	OutboxID  int64           `json:"outbox_id"`
	UserID    int             `json:"user_id"`
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload"`
}

// Message adalah pesan yang siap dikirim, tidak bergantung pada cara pengiriman.
type Message struct {
	To      string
	Subject string
	Body    string
}

// Notification adalah notifikasi yang sudah terkirim.
type Notification struct {
	ID       int64     `json:"id"`
	OutboxID int64     `json:"outbox_id"`
	UserID   int       `json:"user_id"`
	Channel  string    `json:"channel"`
	Subject  string    `json:"subject"`
	Body     string    `json:"body"`
	SentAt   time.Time `json:"sent_at"`
}

// QueueStats adalah ringkasan keadaan untuk admin.
type QueueStats struct {
	OutboxPending     int        `json:"outbox_pending"`
	OutboxPublished   int        `json:"outbox_published"`
	OldestPendingAge  string     `json:"oldest_pending_age"`
	NotificationsSent int        `json:"notifications_sent"`
	Queue             *QueueInfo `json:"queue"`
	QueueError        string     `json:"queue_error,omitempty"`
}

// QueueInfo adalah salinan sebagian informasi queue dari Asynq.
type QueueInfo struct {
	Name      string `json:"name"`
	Pending   int    `json:"pending"`
	Active    int    `json:"active"`
	Scheduled int    `json:"scheduled"`
	Retry     int    `json:"retry"`
	Archived  int    `json:"archived"`
	Completed int    `json:"completed"`
	Processed int    `json:"processed_today"`
	Failed    int    `json:"failed_today"`
}
