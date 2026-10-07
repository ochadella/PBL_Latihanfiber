package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"siakad-mini/app/model"
)

// CourseRepository adalah kontrak akses data mata kuliah.
type CourseRepository interface {
	FindAll(ctx context.Context, f model.CourseFilter) ([]model.Course, error)
}

type coursePostgresRepository struct {
	pool *pgxpool.Pool
}

func NewCourseRepository(pool *pgxpool.Pool) CourseRepository {
	return &coursePostgresRepository{pool: pool}
}

func (r *coursePostgresRepository) FindAll(ctx context.Context, f model.CourseFilter) ([]model.Course, error) {
	// "terisi" dihitung langsung dari tabel enrollments.
	query := `
		SELECT c.id, c.kode_mk, c.nama_mk, c.sks, c.semester, c.kuota,
		       COUNT(e.id) AS terisi
		FROM courses c
		LEFT JOIN enrollments e ON e.course_id = c.id`

	// WHERE disusun dinamis. Nilai filter TIDAK pernah ditempel langsung
	// ke string SQL, selalu lewat placeholder $1, $2, ... (aman dari SQL injection).
	var conditions []string
	var args []any

	if f.Semester > 0 {
		args = append(args, f.Semester)
		conditions = append(conditions, fmt.Sprintf("c.semester = $%d", len(args)))
	}
	if f.Search != "" {
		args = append(args, "%"+f.Search+"%")
		conditions = append(conditions,
			fmt.Sprintf("(c.kode_mk ILIKE $%d OR c.nama_mk ILIKE $%d)", len(args), len(args)))
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += " GROUP BY c.id"
	if f.Available {
		query += " HAVING COUNT(e.id) < c.kuota"
	}
	query += " ORDER BY c.semester, c.kode_mk"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	courses := make([]model.Course, 0) // supaya hasil kosong menjadi [] bukan null
	for rows.Next() {
		var c model.Course
		if err := rows.Scan(&c.ID, &c.KodeMK, &c.NamaMK, &c.SKS, &c.Semester, &c.Kuota, &c.Terisi); err != nil {
			return nil, err
		}
		c.SisaKuota = c.Kuota - c.Terisi
		if c.SisaKuota < 0 {
			c.SisaKuota = 0
		}
		courses = append(courses, c)
	}
	return courses, rows.Err()
}