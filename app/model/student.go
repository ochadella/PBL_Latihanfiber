package model

import "time"

// Student adalah data mahasiswa (tabel students + email dari tabel users).
type Student struct {
	ID          int       `json:"id"`
	UserID      int       `json:"user_id"`
	NIM         string    `json:"nim"`
	Nama        string    `json:"nama"`
	Email       string    `json:"email"`
	Prodi       string    `json:"prodi"`
	Angkatan    int       `json:"angkatan"`
	IPKTerakhir float64   `json:"ipk_terakhir"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// EnrolledCourse adalah satu mata kuliah yang diambil mahasiswa (isi KRS).
type EnrolledCourse struct {
	EnrollmentID  int    `json:"enrollment_id"`
	CourseID      int    `json:"course_id"`
	KodeMK        string `json:"kode_mk"`
	NamaMK        string `json:"nama_mk"`
	SKS           int    `json:"sks"`
	Semester      int    `json:"semester"`
	TahunAkademik string `json:"tahun_akademik"`
}

// StudentDetail adalah response GET /api/v1/students/{id}.
type StudentDetail struct {
	Student
	MataKuliah []EnrolledCourse `json:"mata_kuliah"`
	TotalSKS   int              `json:"total_sks"`
	BatasSKS   int              `json:"batas_sks"`
}

// StudentFilter berisi query parameter GET /api/v1/students.
type StudentFilter struct {
	Page     int
	PerPage  int
	Prodi    string
	Angkatan int
	Search   string
	Sort     string
}

// CreateStudentRequest adalah body POST /api/v1/students.
// Field berupa pointer supaya bisa dibedakan antara "tidak dikirim" dan "dikirim kosong".
type CreateStudentRequest struct {
	NIM         *string  `json:"nim"`
	Nama        *string  `json:"nama"`
	Email       *string  `json:"email"`
	Prodi       *string  `json:"prodi"`
	Angkatan    *int     `json:"angkatan"`
	IPKTerakhir *float64 `json:"ipk_terakhir"`
}

// UpdateStudentRequest adalah body PUT /api/v1/students/{id}.
// NIM sengaja ada di sini hanya untuk DITOLAK bila dikirim (NIM tidak boleh diubah).
type UpdateStudentRequest struct {
	NIM         *string  `json:"nim"`
	Nama        *string  `json:"nama"`
	Prodi       *string  `json:"prodi"`
	Angkatan    *int     `json:"angkatan"`
	IPKTerakhir *float64 `json:"ipk_terakhir"`
}

// NewStudent adalah data bersih yang siap disimpan ke database.
type NewStudent struct {
	NIM          string
	Nama         string
	Email        string
	Prodi        string
	Angkatan     int
	IPKTerakhir  float64
	PasswordHash string
}

// StudentUpdate adalah data bersih untuk mengubah mahasiswa.
type StudentUpdate struct {
	Nama        string
	Prodi       string
	Angkatan    int
	IPKTerakhir *float64 // nil = tidak diubah
}