package middleware

import (
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"

	"siakad-mini/helper"
)

// RequireAuth memastikan request membawa header "Authorization: Bearer <token>" yang valid.
func RequireAuth(jwtManager *helper.JWTManager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		header := c.Get(fiber.HeaderAuthorization)
		if header == "" {
			return helper.Fail(c, fiber.StatusUnauthorized, "Token tidak ditemukan")
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
			return helper.Fail(c, fiber.StatusUnauthorized, "Format token harus: Bearer <token>")
		}

		authUser, err := jwtManager.Parse(strings.TrimSpace(parts[1]))
		if errors.Is(err, helper.ErrTokenExpired) {
			return helper.Fail(c, fiber.StatusUnauthorized, "Token sudah kedaluwarsa")
		}
		if err != nil {
			return helper.Fail(c, fiber.StatusUnauthorized, "Token tidak valid")
		}

		helper.SetAuthUser(c, authUser)
		return c.Next()
	}
}

// RequireRole hanya mengizinkan role tertentu. Dipasang SETELAH RequireAuth.
func RequireRole(roles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		user, ok := helper.CurrentUser(c)
		if !ok {
			return helper.Fail(c, fiber.StatusUnauthorized, "Token tidak valid")
		}
		for _, r := range roles {
			if user.Role == r {
				return c.Next()
			}
		}
		return helper.Fail(c, fiber.StatusForbidden, "Anda tidak memiliki akses ke resource ini")
	}
}

// LoginRateLimiter: maksimal 5 percobaan login GAGAL per menit per alamat IP.
// Percobaan ke-6 dibalas 429.
func LoginRateLimiter() fiber.Handler {
	return limiter.New(limiter.Config{
		Max:                    5,
		Expiration:             1 * time.Minute,
		SkipSuccessfulRequests: true, // login yang berhasil tidak dihitung
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return helper.Fail(c, fiber.StatusTooManyRequests,
				"Terlalu banyak percobaan login gagal. Coba lagi dalam 1 menit")
		},
	})
}
