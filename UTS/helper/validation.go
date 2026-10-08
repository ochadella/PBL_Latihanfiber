package helper

import "regexp"

var emailRegex = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// IsValidEmail memeriksa format email sederhana.
func IsValidEmail(email string) bool {
	return emailRegex.MatchString(email)
}

// ValidationErrors menampung pesan error per field, contoh:
// {"email": ["Email wajib diisi"], "password": ["Password minimal 8 karakter"]}
type ValidationErrors map[string][]string

// Add menambahkan satu pesan error untuk sebuah field.
func (v ValidationErrors) Add(field, message string) {
	v[field] = append(v[field], message)
}

// HasErrors bernilai true bila ada minimal satu error.
func (v ValidationErrors) HasErrors() bool {
	return len(v) > 0
}
