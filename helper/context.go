package helper

import (
	"github.com/gofiber/fiber/v2"

	"siakad-mini/app/model"
)

const authUserKey = "auth_user"

// SetAuthUser menyimpan identitas user yang login ke dalam request (dipakai middleware).
func SetAuthUser(c *fiber.Ctx, u model.AuthUser) {
	c.Locals(authUserKey, u)
}

// CurrentUser mengambil identitas user yang sedang login.
func CurrentUser(c *fiber.Ctx) (model.AuthUser, bool) {
	u, ok := c.Locals(authUserKey).(model.AuthUser)
	return u, ok
}
