package route

import (
	"github.com/gofiber/fiber/v2"

	"siakad-mini/app/model"
	"siakad-mini/app/service"
	"siakad-mini/helper"
	"siakad-mini/middleware"
)

// Deps berisi semua service yang dibutuhkan route.
type Deps struct {
	JWT            *helper.JWTManager
	AuthService    *service.AuthService
	CourseService  *service.CourseService
	StudentService *service.StudentService
}

// Register mendaftarkan seluruh endpoint API.
func Register(app *fiber.App, deps Deps) {
	api := app.Group("/api/v1")

	api.Get("/health", func(c *fiber.Ctx) error {
		return helper.OK(c, "Server berjalan", nil)
	})

	requireAuth := middleware.RequireAuth(deps.JWT)
	adminOnly := middleware.RequireRole(model.RoleAdmin)
	adminOrMahasiswa := middleware.RequireRole(model.RoleAdmin, model.RoleMahasiswa)

	// ---------- AUTH ----------
	auth := api.Group("/auth")
	auth.Post("/login", middleware.LoginRateLimiter(), deps.AuthService.Login)
	auth.Get("/me", requireAuth, deps.AuthService.Me)

	// ---------- STUDENTS ----------
	students := api.Group("/students", requireAuth)
	students.Get("/", adminOnly, deps.StudentService.List)
	students.Post("/", adminOnly, deps.StudentService.Create)
	students.Get("/:id", adminOrMahasiswa, deps.StudentService.Detail) // mahasiswa: hanya data sendiri
	students.Put("/:id", adminOnly, deps.StudentService.Update)
	students.Delete("/:id", adminOnly, deps.StudentService.Delete)

	// ---------- COURSES (semua role) ----------
	api.Get("/courses", requireAuth, deps.CourseService.List)

	// Endpoint enrollments ditambahkan di langkah berikutnya.

	// Route yang tidak terdaftar -> 404 dengan format JSON seragam.
	app.Use(func(c *fiber.Ctx) error {
		return helper.Fail(c, fiber.StatusNotFound, "Endpoint tidak ditemukan")
	})
}