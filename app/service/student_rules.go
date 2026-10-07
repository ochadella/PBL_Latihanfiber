package service

import (
	"regexp"
	"strings"

	"siakad-mini/app/model"
	"siakad-mini/helper"
)

// File ini berisi business rules murni (tidak mengimpor Fiber),
// sehingga bisa dipakai ulang dan mudah dibuat unit test-nya.

var nimRegex = regexp.MustCompile(`^[0-9]{12}$`)

// BatasSKS mengembalikan batas SKS per semester berdasarkan IPK terakhir.
//
//	IPK >= 3.00        -> 24 SKS
//	IPK 2.50 - 2.99    -> 21 SKS
//	IPK <  2.50        -> 18 SKS
func BatasSKS(ipk float64) int {
	switch {
	case ipk >= 3.00:
		return 24
	case ipk >= 2.50:
		return 21
	default:
		return 18
	}
}

// validateStudentFields memeriksa field yang sama untuk POST dan PUT.
func validateStudentFields(errs helper.ValidationErrors, nama, prodi *string, angkatan *int, ipk *float64, currentYear int) {
	if nama == nil || strings.TrimSpace(*nama) == "" {
		errs.Add("nama", "Nama wajib diisi")
	} else if len(strings.TrimSpace(*nama)) > 100 {
		errs.Add("nama", "Nama maksimal 100 karakter")
	}

	if prodi == nil || strings.TrimSpace(*prodi) == "" {
		errs.Add("prodi", "Prodi wajib diisi")
	} else if len(strings.TrimSpace(*prodi)) > 100 {
		errs.Add("prodi", "Prodi maksimal 100 karakter")
	}

	if angkatan == nil {
		errs.Add("angkatan", "Angkatan wajib diisi")
	} else if *angkatan < 1000 || *angkatan > 9999 {
		errs.Add("angkatan", "Angkatan harus 4 digit")
	} else if *angkatan > currentYear {
		errs.Add("angkatan", "Angkatan tidak boleh melebihi tahun berjalan")
	}

	if ipk != nil && (*ipk < 0 || *ipk > 4) {
		errs.Add("ipk_terakhir", "IPK terakhir harus di antara 0.00 dan 4.00")
	}
}

// ValidateCreateStudent memeriksa body POST /api/v1/students.
func ValidateCreateStudent(req model.CreateStudentRequest, currentYear int) helper.ValidationErrors {
	errs := helper.ValidationErrors{}

	if req.NIM == nil || strings.TrimSpace(*req.NIM) == "" {
		errs.Add("nim", "NIM wajib diisi")
	} else if !nimRegex.MatchString(strings.TrimSpace(*req.NIM)) {
		errs.Add("nim", "NIM harus terdiri dari 12 digit angka")
	}

	if req.Email == nil || strings.TrimSpace(*req.Email) == "" {
		errs.Add("email", "Email wajib diisi")
	} else if !helper.IsValidEmail(strings.TrimSpace(*req.Email)) {
		errs.Add("email", "Format email tidak valid")
	}

	validateStudentFields(errs, req.Nama, req.Prodi, req.Angkatan, req.IPKTerakhir, currentYear)
	return errs
}

// ValidateUpdateStudent memeriksa body PUT /api/v1/students/{id}.
func ValidateUpdateStudent(req model.UpdateStudentRequest, currentYear int) helper.ValidationErrors {
	errs := helper.ValidationErrors{}

	if req.NIM != nil {
		errs.Add("nim", "NIM tidak boleh diubah")
	}

	validateStudentFields(errs, req.Nama, req.Prodi, req.Angkatan, req.IPKTerakhir, currentYear)
	return errs
}