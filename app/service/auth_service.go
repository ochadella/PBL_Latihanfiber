package service

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"

	"siakad-mini/app/model"
	"siakad-mini/app/repository"
	"siakad-mini/helper"
)

type AuthService struct {
	users repository.UserRepository
	jwt   *helper.JWTManager
}

func NewAuthService(users repository.UserRepository, jwt *helper.JWTManager) *AuthService {
	return &AuthService{users: users, jwt: jwt}
}

// Login -> POST /api/v1/auth/login
func (s *AuthService) Login(c *fiber.Ctx) error {
	var req model.LoginRequest
	// Body dibaca langsung sebagai JSON (tidak bergantung pada header Content-Type).
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return helper.FailValidation(c, map[string][]string{
			"body": {"Body harus berupa JSON yang valid"},
		})
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	if errs := ValidateLogin(req); errs.HasErrors() {
		return helper.FailValidation(c, errs)
	}

	user, err := s.users.FindActiveByEmail(c.UserContext(), req.Email)
	if errors.Is(err, repository.ErrNotFound) {
		// Pesan sengaja sama dengan "password salah" agar penyerang
		// tidak bisa menebak email mana yang terdaftar.
		return helper.Fail(c, fiber.StatusUnauthorized, "Email atau password salah")
	}
	if err != nil {
		return err // diteruskan ke ErrorHandler -> 500
	}

	if !helper.CheckPassword(user.Password, req.Password) {
		return helper.Fail(c, fiber.StatusUnauthorized, "Email atau password salah")
	}

	token, err := s.jwt.GenerateAccess(user)
	if err != nil {
		return err
	}

	return helper.OK(c, "Login berhasil", model.LoginResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   int(s.jwt.AccessTTL().Seconds()),
		User:        model.UserResponse{ID: user.ID, Email: user.Email, Role: user.Role},
	})
}

// Me -> GET /api/v1/auth/me
func (s *AuthService) Me(c *fiber.Ctx) error {
	authUser, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Fail(c, fiber.StatusUnauthorized, "Token tidak valid")
	}

	user, err := s.users.FindActiveByID(c.UserContext(), authUser.ID)
	if errors.Is(err, repository.ErrNotFound) {
		// Contoh: akun mahasiswa sudah di-soft delete setelah token dibuat.
		return helper.Fail(c, fiber.StatusUnauthorized, "Akun tidak aktif atau tidak ditemukan")
	}
	if err != nil {
		return err
	}

	resp := model.MeResponse{ID: user.ID, Email: user.Email, Role: user.Role}

	if user.Role == model.RoleMahasiswa {
		student, err := s.users.FindStudentBriefByUserID(c.UserContext(), user.ID)
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			return err
		}
		resp.Student = student
	}

	return helper.OK(c, "Profil pengguna berhasil diambil", resp)
}