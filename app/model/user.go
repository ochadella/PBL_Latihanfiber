package model

import "time"

const (
	RoleAdmin     = "admin"
	RoleMahasiswa = "mahasiswa"
)

// User adalah satu baris tabel users.
type User struct {
	ID        int       `json:"id"`
	Email     string    `json:"email"`
	Password  string    `json:"-"` // tanda "-" = tidak pernah ikut dikirim ke JSON
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// AuthUser adalah identitas ringkas yang dibaca dari token.
type AuthUser struct {
	ID    int
	Email string
	Role  string
}
