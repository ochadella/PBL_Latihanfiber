package model

import "time"

// CreateEnrollmentRequest adalah body POST /api/v1/enrollments.
type CreateEnrollmentRequest struct {
	CourseID      *int    `json:"course_id"`
	TahunAkademik *string `json:"tahun_akademik"`
}

// Enrollment adalah satu baris KRS (mata kuliah yang diambil mahasiswa).
type Enrollment struct {
	ID            int       `json:"id"`
	StudentID     int       `json:"student_id"`
	CourseID      int       `json:"course_id"`
	KodeMK        string    `json:"kode_mk"`
	NamaMK        string    `json:"nama_mk"`
	SKS           int       `json:"sks"`
	TahunAkademik string    `json:"tahun_akademik"`
	CreatedAt     time.Time `json:"created_at"`
}

// EnrollmentResult adalah response POST /api/v1/enrollments.
type EnrollmentResult struct {
	Enrollment
	TotalSKS int `json:"total_sks"` // total SKS di tahun akademik tsb setelah menambah
	BatasSKS int `json:"batas_sks"`
	SisaSKS  int `json:"sisa_sks"`
}