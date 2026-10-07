package model

// Course adalah data mata kuliah beserta informasi kuota.
type Course struct {
	ID        int    `json:"id"`
	KodeMK    string `json:"kode_mk"`
	NamaMK    string `json:"nama_mk"`
	SKS       int    `json:"sks"`
	Semester  int    `json:"semester"`
	Kuota     int    `json:"kuota"`
	Terisi    int    `json:"terisi"`     // jumlah baris di tabel enrollments
	SisaKuota int    `json:"sisa_kuota"` // kuota - terisi
}

// CourseFilter berisi query parameter untuk GET /api/v1/courses.
type CourseFilter struct {
	Semester  int    // 0 = tidak difilter
	Search    string // cari di kode_mk atau nama_mk
	Available bool   // true = hanya yang kuotanya belum penuh
}