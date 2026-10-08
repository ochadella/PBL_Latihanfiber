package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"siakad-mini/app/model"
)

var (
	ErrCourseNotFound   = errors.New("mata kuliah tidak ditemukan")
	ErrAlreadyEnrolled  = errors.New("mata kuliah sudah diambil")
	ErrQuotaFull        = errors.New("kuota mata kuliah penuh")
	ErrStudentNotActive = errors.New("data mahasiswa tidak aktif")
)

// SKSLimitError dikembalikan bila total SKS melebihi batas.
type SKSLimitError struct {
	Batas     int // batas SKS sesuai IPK
	Diambil   int // SKS yang sudah diambil di tahun akademik tsb
	SKSMatkul int // SKS mata kuliah yang ingin diambil
}

func (e *SKSLimitError) Error() string {
	return fmt.Sprintf("total SKS melebihi batas (batas %d, diambil %d, mata kuliah %d)",
		e.Batas, e.Diambil, e.SKSMatkul)
}

// Sisa adalah SKS yang masih boleh diambil.
func (e *SKSLimitError) Sisa() int {
	if e.Batas-e.Diambil < 0 {
		return 0
	}
	return e.Batas - e.Diambil
}

// EnrollmentRepository adalah kontrak akses data KRS.
type EnrollmentRepository interface {
	FindStudentIDByUserID(ctx context.Context, userID int) (int, error)
	Enroll(ctx context.Context, studentID, courseID int, tahun string,
		batasSKS func(ipk float64) int) (model.EnrollmentResult, error)
	FindByID(ctx context.Context, id int) (model.Enrollment, error)
	Delete(ctx context.Context, id int) error
}

type enrollmentPostgresRepository struct {
	pool *pgxpool.Pool
}

func NewEnrollmentRepository(pool *pgxpool.Pool) EnrollmentRepository {
	return &enrollmentPostgresRepository{pool: pool}
}

func (r *enrollmentPostgresRepository) FindStudentIDByUserID(ctx context.Context, userID int) (int, error) {
	var id int
	err := r.pool.QueryRow(ctx,
		`SELECT id FROM students WHERE user_id = $1 AND deleted_at IS NULL`, userID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrStudentNotActive
	}
	return id, err
}

// Enroll menjalankan seluruh pengecekan + insert dalam SATU transaction.
//
// Row locking (SELECT ... FOR UPDATE):
//   - baris students dikunci  -> 2 request dari mahasiswa yang sama tidak bisa
//     sama-sama lolos cek batas SKS.
//   - baris courses dikunci   -> 2 mahasiswa yang berebut kursi terakhir harus
//     antre; yang kedua baru jalan setelah yang pertama selesai, sehingga
//     kuota tidak pernah terlampaui.
func (r *enrollmentPostgresRepository) Enroll(ctx context.Context, studentID, courseID int,
	tahun string, batasSKS func(ipk float64) int) (model.EnrollmentResult, error) {

	var res model.EnrollmentResult

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return res, err
	}
	defer tx.Rollback(ctx) // tidak berpengaruh bila sudah Commit

	// 1. Kunci data mahasiswa + ambil IPK
	var ipk float64
	err = tx.QueryRow(ctx,
		`SELECT ipk_terakhir FROM students
		 WHERE id = $1 AND deleted_at IS NULL
		 FOR UPDATE`, studentID).Scan(&ipk)
	if errors.Is(err, pgx.ErrNoRows) {
		return res, ErrStudentNotActive
	}
	if err != nil {
		return res, err
	}

	// 2. Kunci mata kuliah + ambil data
	var kuota int
	err = tx.QueryRow(ctx,
		`SELECT kode_mk, nama_mk, sks, kuota FROM courses
		 WHERE id = $1
		 FOR UPDATE`, courseID).Scan(&res.KodeMK, &res.NamaMK, &res.SKS, &kuota)
	if errors.Is(err, pgx.ErrNoRows) {
		return res, ErrCourseNotFound
	}
	if err != nil {
		return res, err
	}

	// 3. Cek duplikasi (business rule 2)
	var sudahAmbil bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS (
			SELECT 1 FROM enrollments
			WHERE student_id = $1 AND course_id = $2 AND tahun_akademik = $3
		 )`, studentID, courseID, tahun).Scan(&sudahAmbil); err != nil {
		return res, err
	}
	if sudahAmbil {
		return res, ErrAlreadyEnrolled
	}

	// 4. Cek kuota (business rule 3)
	var terisi int
	if err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM enrollments WHERE course_id = $1`, courseID).Scan(&terisi); err != nil {
		return res, err
	}
	if terisi >= kuota {
		return res, ErrQuotaFull
	}

	// 5. Cek batas SKS (business rule 1)
	var diambil int
	if err := tx.QueryRow(ctx,
		`SELECT COALESCE(SUM(c.sks), 0)
		 FROM enrollments e JOIN courses c ON c.id = e.course_id
		 WHERE e.student_id = $1 AND e.tahun_akademik = $2`,
		studentID, tahun).Scan(&diambil); err != nil {
		return res, err
	}
	batas := batasSKS(ipk)
	if diambil+res.SKS > batas {
		return res, &SKSLimitError{Batas: batas, Diambil: diambil, SKSMatkul: res.SKS}
	}

	// 6. Simpan
	err = tx.QueryRow(ctx,
		`INSERT INTO enrollments (student_id, course_id, tahun_akademik)
		 VALUES ($1, $2, $3)
		 RETURNING id, created_at`,
		studentID, courseID, tahun).Scan(&res.ID, &res.CreatedAt)
	if err != nil {
		if errors.Is(mapDuplicate(err), ErrDuplicate) {
			return res, ErrAlreadyEnrolled
		}
		return res, err
	}

	if err := tx.Commit(ctx); err != nil {
		return res, err
	}

	res.StudentID = studentID
	res.CourseID = courseID
	res.TahunAkademik = tahun
	res.TotalSKS = diambil + res.SKS
	res.BatasSKS = batas
	res.SisaSKS = batas - res.TotalSKS
	return res, nil
}

func (r *enrollmentPostgresRepository) FindByID(ctx context.Context, id int) (model.Enrollment, error) {
	var e model.Enrollment
	err := r.pool.QueryRow(ctx,
		`SELECT e.id, e.student_id, e.course_id, c.kode_mk, c.nama_mk, c.sks,
		        e.tahun_akademik, e.created_at
		 FROM enrollments e JOIN courses c ON c.id = e.course_id
		 WHERE e.id = $1`, id,
	).Scan(&e.ID, &e.StudentID, &e.CourseID, &e.KodeMK, &e.NamaMK, &e.SKS,
		&e.TahunAkademik, &e.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Enrollment{}, ErrNotFound
	}
	return e, err
}

func (r *enrollmentPostgresRepository) Delete(ctx context.Context, id int) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM enrollments WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}