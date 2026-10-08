package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"siakad-mini/app/model"
)

// UserRepository adalah kontrak akses data user (tanpa detail SQL).
type UserRepository interface {
	FindActiveByEmail(ctx context.Context, email string) (model.User, error)
	FindActiveByID(ctx context.Context, id int) (model.User, error)
	FindStudentBriefByUserID(ctx context.Context, userID int) (*model.StudentBrief, error)
}

type userPostgresRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) UserRepository {
	return &userPostgresRepository{pool: pool}
}

// Kondisi "aktif": user BUKAN mahasiswa yang sudah di-soft delete.
const activeUserCondition = `NOT EXISTS (
	SELECT 1 FROM students s WHERE s.user_id = u.id AND s.deleted_at IS NOT NULL
)`

func (r *userPostgresRepository) FindActiveByEmail(ctx context.Context, email string) (model.User, error) {
	var u model.User
	err := r.pool.QueryRow(ctx,
		`SELECT u.id, u.email, u.password, u.role, u.created_at
		 FROM users u
		 WHERE u.email = $1 AND `+activeUserCondition,
		email,
	).Scan(&u.ID, &u.Email, &u.Password, &u.Role, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.User{}, ErrNotFound
	}
	return u, err
}

func (r *userPostgresRepository) FindActiveByID(ctx context.Context, id int) (model.User, error) {
	var u model.User
	err := r.pool.QueryRow(ctx,
		`SELECT u.id, u.email, u.password, u.role, u.created_at
		 FROM users u
		 WHERE u.id = $1 AND `+activeUserCondition,
		id,
	).Scan(&u.ID, &u.Email, &u.Password, &u.Role, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.User{}, ErrNotFound
	}
	return u, err
}

func (r *userPostgresRepository) FindStudentBriefByUserID(ctx context.Context, userID int) (*model.StudentBrief, error) {
	var s model.StudentBrief
	err := r.pool.QueryRow(ctx,
		`SELECT id, nim, nama, prodi, angkatan
		 FROM students
		 WHERE user_id = $1 AND deleted_at IS NULL`,
		userID,
	).Scan(&s.ID, &s.NIM, &s.Nama, &s.Prodi, &s.Angkatan)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}
