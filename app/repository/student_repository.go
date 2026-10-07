package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"siakad-mini/app/model"
)

// ErrDuplicate dipakai bila data melanggar UNIQUE (kode PostgreSQL 23505).
var ErrDuplicate = errors.New("data duplikat")

// StudentRepository adalah kontrak akses data mahasiswa.
type StudentRepository interface {
	List(ctx context.Context, f model.StudentFilter) ([]model.Student, int, error)
	FindByID(ctx context.Context, id int) (model.Student, error)
	NIMExists(ctx context.Context, nim string) (bool, error)
	EmailExists(ctx context.Context, email string) (bool, error)
	Create(ctx context.Context, s model.NewStudent) (model.Student, error)
	Update(ctx context.Context, id int, u model.StudentUpdate) (model.Student, error)
	SoftDelete(ctx context.Context, id int) error
	FindEnrolledCourses(ctx context.Context, studentID int) ([]model.EnrolledCourse, error)
}

type studentPostgresRepository struct {
	pool *pgxpool.Pool
}

func NewStudentRepository(pool *pgxpool.Pool) StudentRepository {
	return &studentPostgresRepository{pool: pool}
}

// Kolom yang selalu dibaca untuk satu mahasiswa.
const studentColumns = `s.id, s.user_id, s.nim, s.nama, u.email, s.prodi, s.angkatan,
	s.ipk_terakhir, s.created_at, s.updated_at`

func scanStudent(row pgx.Row) (model.Student, error) {
	var s model.Student
	err := row.Scan(&s.ID, &s.UserID, &s.NIM, &s.Nama, &s.Email, &s.Prodi, &s.Angkatan,
		&s.IPKTerakhir, &s.CreatedAt, &s.UpdatedAt)
	return s, err
}

// Whitelist kolom sort: nilai dari klien TIDAK pernah ditempel langsung ke SQL.
var studentSortColumns = map[string]string{
	"nama":          "s.nama ASC",
	"-nama":         "s.nama DESC",
	"ipk_terakhir":  "s.ipk_terakhir ASC",
	"-ipk_terakhir": "s.ipk_terakhir DESC",
}

func (r *studentPostgresRepository) List(ctx context.Context, f model.StudentFilter) ([]model.Student, int, error) {
	conditions := []string{"s.deleted_at IS NULL"} // yang sudah di-soft delete tidak tampil
	var args []any

	if f.Prodi != "" {
		args = append(args, f.Prodi)
		conditions = append(conditions, fmt.Sprintf("s.prodi ILIKE $%d", len(args)))
	}
	if f.Angkatan > 0 {
		args = append(args, f.Angkatan)
		conditions = append(conditions, fmt.Sprintf("s.angkatan = $%d", len(args)))
	}
	if f.Search != "" {
		args = append(args, "%"+f.Search+"%")
		conditions = append(conditions,
			fmt.Sprintf("(s.nim ILIKE $%d OR s.nama ILIKE $%d)", len(args), len(args)))
	}
	where := " WHERE " + strings.Join(conditions, " AND ")

	// 1) Hitung total data (untuk meta pagination)
	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM students s`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// 2) Ambil data halaman yang diminta
	orderBy := "s.id ASC"
	if col, ok := studentSortColumns[f.Sort]; ok {
		orderBy = col + ", s.id ASC"
	}
	args = append(args, f.PerPage, (f.Page-1)*f.PerPage)
	query := `SELECT ` + studentColumns + `
		FROM students s JOIN users u ON u.id = s.user_id` + where +
		` ORDER BY ` + orderBy +
		fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)-1, len(args))

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	students := make([]model.Student, 0)
	for rows.Next() {
		s, err := scanStudent(rows)
		if err != nil {
			return nil, 0, err
		}
		students = append(students, s)
	}
	return students, total, rows.Err()
}

func (r *studentPostgresRepository) FindByID(ctx context.Context, id int) (model.Student, error) {
	s, err := scanStudent(r.pool.QueryRow(ctx,
		`SELECT `+studentColumns+`
		 FROM students s JOIN users u ON u.id = s.user_id
		 WHERE s.id = $1 AND s.deleted_at IS NULL`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Student{}, ErrNotFound
	}
	return s, err
}

func (r *studentPostgresRepository) NIMExists(ctx context.Context, nim string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM students WHERE nim = $1)`, nim).Scan(&exists)
	return exists, err
}

func (r *studentPostgresRepository) EmailExists(ctx context.Context, email string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM users WHERE email = $1)`, email).Scan(&exists)
	return exists, err
}

// Create membuat akun users + data students dalam SATU transaction.
// Bila salah satu gagal, keduanya dibatalkan (rollback).
func (r *studentPostgresRepository) Create(ctx context.Context, s model.NewStudent) (model.Student, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return model.Student{}, err
	}
	defer tx.Rollback(ctx) // tidak berpengaruh bila sudah Commit

	var userID int
	err = tx.QueryRow(ctx,
		`INSERT INTO users (email, password, role) VALUES ($1, $2, 'mahasiswa') RETURNING id`,
		s.Email, s.PasswordHash).Scan(&userID)
	if err != nil {
		return model.Student{}, mapDuplicate(err)
	}

	var studentID int
	err = tx.QueryRow(ctx,
		`INSERT INTO students (user_id, nim, nama, prodi, angkatan, ipk_terakhir)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		userID, s.NIM, s.Nama, s.Prodi, s.Angkatan, s.IPKTerakhir).Scan(&studentID)
	if err != nil {
		return model.Student{}, mapDuplicate(err)
	}

	if err := tx.Commit(ctx); err != nil {
		return model.Student{}, err
	}
	return r.FindByID(ctx, studentID)
}

func (r *studentPostgresRepository) Update(ctx context.Context, id int, u model.StudentUpdate) (model.Student, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE students
		 SET nama = $1, prodi = $2, angkatan = $3,
		     ipk_terakhir = COALESCE($4, ipk_terakhir),
		     updated_at = NOW()
		 WHERE id = $5 AND deleted_at IS NULL`,
		u.Nama, u.Prodi, u.Angkatan, u.IPKTerakhir, id)
	if err != nil {
		return model.Student{}, err
	}
	if tag.RowsAffected() == 0 {
		return model.Student{}, ErrNotFound
	}
	return r.FindByID(ctx, id)
}

// SoftDelete tidak menghapus baris, hanya mengisi deleted_at.
func (r *studentPostgresRepository) SoftDelete(ctx context.Context, id int) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE students SET deleted_at = NOW(), updated_at = NOW()
		 WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *studentPostgresRepository) FindEnrolledCourses(ctx context.Context, studentID int) ([]model.EnrolledCourse, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT e.id, c.id, c.kode_mk, c.nama_mk, c.sks, c.semester, e.tahun_akademik
		 FROM enrollments e JOIN courses c ON c.id = e.course_id
		 WHERE e.student_id = $1
		 ORDER BY e.tahun_akademik, c.kode_mk`, studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	courses := make([]model.EnrolledCourse, 0)
	for rows.Next() {
		var ec model.EnrolledCourse
		if err := rows.Scan(&ec.EnrollmentID, &ec.CourseID, &ec.KodeMK, &ec.NamaMK,
			&ec.SKS, &ec.Semester, &ec.TahunAkademik); err != nil {
			return nil, err
		}
		courses = append(courses, ec)
	}
	return courses, rows.Err()
}

// mapDuplicate mengubah error UNIQUE PostgreSQL (23505) menjadi ErrDuplicate.
func mapDuplicate(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrDuplicate
	}
	return err
}