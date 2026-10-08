package service

import (
	"strings"

	"latihan-fiber/app/model"
)

// File ini berisi business rules MURNI: tidak menyentuh fiber.Ctx,
// tidak menyentuh database, dan tidak tahu apa pun tentang HTTP.
//
// Modul 7: ValidateCreate, ValidateReplace, dan isValidEmail DIHAPUS.
// Aturan bentuk data kini ditulis sebagai tag validate pada struct.
// Yang tersisa di sini hanyalah aturan yang memang tidak dapat
// dinyatakan sebagai tag.

// ApplyPatch tidak lagi mengembalikan daftar error. Pemeriksaan bentuk
// sudah selesai dikerjakan tag sebelum fungsi ini dipanggil, sehingga di
// sini tugasnya tinggal satu: menggabungkan.
func ApplyPatch(current model.User, req model.PatchUserRequest) model.User {
	if req.Username != nil {
		current.Username = strings.TrimSpace(*req.Username)
	}
	if req.Email != nil {
		current.Email = strings.TrimSpace(*req.Email)
	}
	if req.IsActive != nil {
		current.IsActive = *req.IsActive
	}
	return current
}

// IsEmptyPatch memeriksa body PATCH yang tidak berisi field apa pun.
//
// Aturan ini tidak dapat ditulis sebagai tag: tag memeriksa satu field
// pada satu waktu, sedangkan aturan ini berbicara tentang HUBUNGAN antar
// field — setidaknya satu di antara mereka harus ada.
func IsEmptyPatch(req model.PatchUserRequest) bool {
	return req.Username == nil && req.Email == nil && req.IsActive == nil
}

// CountTotalPages membulatkan ke atas tanpa memakai bilangan pecahan.
// (Dipakai pagination offset dari Modul 4; dipertahankan beserta test-nya.)
func CountTotalPages(total, limit int) int {
	if limit <= 0 {
		return 0
	}
	return (total + limit - 1) / limit
}
