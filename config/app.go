package config

import (
	"errors"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"

	"siakad-mini/helper"
)

// NewApp membuat instance Fiber beserta middleware global dan error handler.
func NewApp() *fiber.App {
	app := fiber.New(fiber.Config{
		AppName: "SIAKAD Mini API",
		// Semua error yang tidak ditangani di service berakhir di sini.
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			var fe *fiber.Error
			if errors.As(err, &fe) {
				return helper.Fail(c, fe.Code, fe.Message)
			}
			// Detail error hanya dicatat di log server, TIDAK dikirim ke klien.
			log.Printf("[ERROR] %s %s: %v", c.Method(), c.Path(), err)
			return helper.Fail(c, fiber.StatusInternalServerError, "Terjadi kesalahan pada server")
		},
	})

	app.Use(recover.New()) // panic -> 500, server tidak mati, stack trace tidak bocor
	app.Use(requestid.New())
	app.Use(logger.New(logger.Config{
		Format: "${time} | ${locals:requestid} | ${status} | ${latency} | ${method} ${path}\n",
	}))

	return app
}
