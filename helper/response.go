package helper

import "github.com/gofiber/fiber/v2"

// Response adalah bentuk JSON seragam untuk semua endpoint.
type Response struct {
	Success bool                `json:"success"`
	Message string              `json:"message"`
	Data    any                 `json:"data,omitempty"`
	Meta    *Meta               `json:"meta,omitempty"`
	Errors  map[string][]string `json:"errors,omitempty"`
}

// Meta berisi informasi pagination untuk response berbentuk daftar.
type Meta struct {
	CurrentPage int `json:"current_page"`
	PerPage     int `json:"per_page"`
	Total       int `json:"total"`
	LastPage    int `json:"last_page"`
}

// OK -> 200
func OK(c *fiber.Ctx, message string, data any) error {
	return c.Status(fiber.StatusOK).JSON(Response{Success: true, Message: message, Data: data})
}

// OKList -> 200 + meta pagination
func OKList(c *fiber.Ctx, message string, data any, meta Meta) error {
	return c.Status(fiber.StatusOK).JSON(Response{Success: true, Message: message, Data: data, Meta: &meta})
}

// Created -> 201
func Created(c *fiber.Ctx, message string, data any) error {
	return c.Status(fiber.StatusCreated).JSON(Response{Success: true, Message: message, Data: data})
}

// NoContent -> 204 (tanpa body)
func NoContent(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusNoContent)
}

// Fail -> status error apa pun (401, 403, 404, 409, 429, 500, ...)
func Fail(c *fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(Response{Success: false, Message: message})
}

// FailValidation -> 422 + daftar error per field
func FailValidation(c *fiber.Ctx, errs map[string][]string) error {
	return c.Status(fiber.StatusUnprocessableEntity).JSON(Response{
		Success: false,
		Message: "Validasi gagal",
		Errors:  errs,
	})
}
