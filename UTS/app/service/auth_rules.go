package service

import (
	"strings"

	"siakad-mini/app/model"
	"siakad-mini/helper"
)

// ValidateLogin berisi aturan validasi login. Tidak mengimpor Fiber
// sehingga mudah dibuat unit test-nya.
func ValidateLogin(req model.LoginRequest) helper.ValidationErrors {
	errs := helper.ValidationErrors{}

	if strings.TrimSpace(req.Email) == "" {
		errs.Add("email", "Email wajib diisi")
	} else if !helper.IsValidEmail(req.Email) {
		errs.Add("email", "Format email tidak valid")
	}

	if req.Password == "" {
		errs.Add("password", "Password wajib diisi")
	} else if len(req.Password) < 8 {
		errs.Add("password", "Password minimal 8 karakter")
	}

	return errs
}
