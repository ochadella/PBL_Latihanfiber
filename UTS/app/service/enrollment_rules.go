package service

import (
	"regexp"
	"strconv"
	"strings"

	"siakad-mini/app/model"
	"siakad-mini/helper"
)

// Format tahun akademik: 2026/2027-Ganjil atau 2026/2027-Genap
var tahunAkademikRegex = regexp.MustCompile(`^([0-9]{4})/([0-9]{4})-(Ganjil|Genap)$`)

// IsValidTahunAkademik memeriksa format dan memastikan tahun kedua = tahun pertama + 1.
func IsValidTahunAkademik(s string) bool {
	m := tahunAkademikRegex.FindStringSubmatch(s)
	if m == nil {
		return false
	}
	awal, _ := strconv.Atoi(m[1])
	akhir, _ := strconv.Atoi(m[2])
	return akhir == awal+1
}

// ValidateCreateEnrollment memeriksa body POST /api/v1/enrollments.
func ValidateCreateEnrollment(req model.CreateEnrollmentRequest) helper.ValidationErrors {
	errs := helper.ValidationErrors{}

	if req.CourseID == nil {
		errs.Add("course_id", "course_id wajib diisi")
	} else if *req.CourseID <= 0 {
		errs.Add("course_id", "course_id harus berupa angka positif")
	}

	if req.TahunAkademik == nil || strings.TrimSpace(*req.TahunAkademik) == "" {
		errs.Add("tahun_akademik", "Tahun akademik wajib diisi")
	} else if !IsValidTahunAkademik(strings.TrimSpace(*req.TahunAkademik)) {
		errs.Add("tahun_akademik", "Format tahun akademik harus seperti 2026/2027-Ganjil atau 2026/2027-Genap")
	}

	return errs
}