package model

import "time"

type User struct {
	ID        int       `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Password  string    `json:"-"`    // tetap: tidak pernah keluar sebagai JSON
	Role      string    `json:"role"` // Modul 5
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

// Mulai pertemuan ini, aturan validasi ditulis sebagai tag pada struct.
// Aturan dan bentuk data berada pada baris yang sama, sehingga menambah
// satu field tanpa aturannya menjadi kelalaian yang langsung terlihat.
type CreateUserRequest struct {
	Username string `json:"username" validate:"required,min=3,max=30,alphanum"`
	Email    string `json:"email"    validate:"required,email,max=120"`
	Password string `json:"password" validate:"required,min=8,max=72,nospace"`
}

type ReplaceUserRequest struct {
	Username string `json:"username" validate:"required,min=3,max=30,alphanum"`
	Email    string `json:"email"    validate:"required,email,max=120"`
	IsActive bool   `json:"is_active"`
}

// Pada PATCH, pointer membedakan "tidak dikirim" (nil) dari "dikirim
// bernilai kosong". omitnil dipilih karena ia menyatakan maksud yang
// sebenarnya: lewati hanya bila nil.
//
// PERBAIKAN #2: pada modul, Username bertipe string biasa. Akibatnya
// ApplyPatch dan IsEmptyPatch yang membandingkannya dengan nil ditolak
// compiler, dan omitnil tidak berarti apa pun pada tipe bukan pointer.
type PatchUserRequest struct {
	Username *string `json:"username,omitempty"  validate:"omitnil,min=3,max=30,alphanum"`
	Email    *string `json:"email,omitempty"     validate:"omitnil,email,max=120"`
	IsActive *bool   `json:"is_active,omitempty"`
}

// AssignRoleRequest dipakai endpoint PATCH /users/:id/role (Modul 6).
type AssignRoleRequest struct {
	Role string `json:"role"`
}

// Amplop baku untuk response keberhasilan
type WebResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
	Meta    any    `json:"meta,omitempty"`
}

// ErrorResponse adalah bentuk SATU-SATUNYA untuk response kegagalan.
type ErrorResponse struct {
	Success   bool              `json:"success"`
	Code      string            `json:"code"`
	Message   string            `json:"message"`
	Fields    map[string]string `json:"fields,omitempty"`
	RequestID string            `json:"request_id,omitempty"`
}

type Meta struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// CursorMeta menggantikan Meta pada endpoint yang memakai cursor.
//
// Perhatikan tidak adanya Total dan TotalPages. Keduanya tidak dapat
// disediakan tanpa COUNT(*) atas seluruh tabel — persis biaya yang ingin
// dihindari oleh pagination berbasis cursor.
type CursorMeta struct {
	Limit      int    `json:"limit"`
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
}

type ListQuery struct {
	Page     int
	Limit    int
	Search   string
	Sort     string
	Order    string
	IsActive *bool
}

// Offset menghitung berapa baris yang dilewati untuk halaman ini.
func (q ListQuery) Offset() int {
	return (q.Page - 1) * q.Limit
}

// Cursor adalah posisi terakhir yang sudah dilihat client.
type Cursor struct {
	CreatedAt time.Time
	ID        int
}

// CursorQuery adalah isi query string untuk cursor pagination.
type CursorQuery struct {
	Limit    int
	Search   string
	IsActive *bool
	After    *Cursor // nil berarti halaman pertama
}
