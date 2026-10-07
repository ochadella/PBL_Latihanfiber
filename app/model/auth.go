package model

// LoginRequest adalah body untuk POST /api/v1/auth/login.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// UserResponse adalah data user yang aman ditampilkan.
type UserResponse struct {
	ID    int    `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

// LoginResponse adalah isi "data" ketika login berhasil.
type LoginResponse struct {
	AccessToken string       `json:"access_token"`
	TokenType   string       `json:"token_type"`
	ExpiresIn   int          `json:"expires_in"`
	User        UserResponse `json:"user"`
}

// StudentBrief adalah data mahasiswa ringkas untuk GET /auth/me.
type StudentBrief struct {
	ID       int    `json:"id"`
	NIM      string `json:"nim"`
	Nama     string `json:"nama"`
	Prodi    string `json:"prodi"`
	Angkatan int    `json:"angkatan"`
}

// MeResponse adalah isi "data" untuk GET /api/v1/auth/me.
type MeResponse struct {
	ID      int           `json:"id"`
	Email   string        `json:"email"`
	Role    string        `json:"role"`
	Student *StudentBrief `json:"student,omitempty"` // hanya ada bila role mahasiswa
}
