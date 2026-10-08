package helper

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"latihan-fiber/app/model"
)

// RequestContext memberi timeout untuk setiap operasi database.
func RequestContext(c *fiber.Ctx) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.UserContext(), 5*time.Second)
}

// RequestID membaca id unik request yang dipasang middleware requestid.
func RequestID(c *fiber.Ctx) string {
	id, _ := c.Locals("requestid").(string)
	return id
}

// ParamID membaca parameter :id dari jalur dan memastikan bentuknya benar.
func ParamID(c *fiber.Ctx) (int, bool) {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id < 1 {
		return 0, false
	}
	return id, true
}

const (
	defaultLimit = 10
	maxLimit     = 100
)

// ParseCursorQuery membaca query string untuk cursor pagination.
//
// Berbeda dari versi lama, nilai yang rusak TIDAK diabaikan diam-diam:
// cursor yang tidak dapat dibaca dan is_active yang bukan boolean
// dijawab 400, karena permintaan client memang salah.
func ParseCursorQuery(c *fiber.Ctx) (model.CursorQuery, error) {
	q := model.CursorQuery{
		Limit:  c.QueryInt("limit", defaultLimit),
		Search: strings.TrimSpace(c.Query("search")),
	}

	// Pembatasan limit terjadi di sini: berapa pun yang diminta client,
	// satu halaman tidak pernah lebih dari maxLimit baris.
	if q.Limit < 1 {
		q.Limit = defaultLimit
	}
	if q.Limit > maxLimit {
		q.Limit = maxLimit
	}

	if raw := c.Query("is_active"); raw != "" {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return model.CursorQuery{}, BadRequest("is_active harus bernilai true atau false")
		}
		q.IsActive = &v
	}

	if raw := c.Query("cursor"); raw != "" {
		cursor, err := DecodeCursor(raw)
		if err != nil {
			return model.CursorQuery{}, BadRequest("cursor tidak valid")
		}
		q.After = &cursor
	}

	return q, nil
}
