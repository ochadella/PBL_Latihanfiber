package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"latihan-fiber/app/model"
	"latihan-fiber/app/repository"
	"latihan-fiber/helper"
)

const refreshTokenBytes = 32

type AuthService struct {
	users      repository.UserRepository
	tokens     repository.TokenRepository
	jwt        *helper.JWTManager
	perms      *helper.PermissionSet // BARU (Modul 6)
	refreshTTL time.Duration
}

func NewAuthService(
	users repository.UserRepository,
	tokens repository.TokenRepository,
	jwtManager *helper.JWTManager,
	perms *helper.PermissionSet,
	refreshTTL time.Duration,
) *AuthService {
	return &AuthService{
		users: users, tokens: tokens, jwt: jwtManager,
		perms: perms, refreshTTL: refreshTTL,
	}
}

// Modul 7: setiap kegagalan dikembalikan sebagai *helper.AppError.
// Validasi memakai tag pada struct (auth_rules.go sudah dihapus).
// Register godoc
//
// @Summary      Mendaftarkan akun baru
// @Description  Role akun baru selalu "user". Password minimal 8 karakter dan memuat huruf serta angka.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      model.RegisterRequest  true  "Data akun baru"
// @Success      201   {object}  model.WebResponse{data=model.User}
// @Failure      400   {object}  model.ErrorResponse  "Body bukan JSON yang valid"
// @Failure      409   {object}  model.ErrorResponse  "Username sudah dipakai"
// @Failure      422   {object}  model.ErrorResponse  "Validasi gagal; detail per field ada di fields"
// @Router       /auth/register [post]
func (s *AuthService) Register(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	var req model.RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)

	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}

	// Password DI-HASH sebelum menyentuh database. Nilai aslinya tidak
	// pernah disimpan, tidak pernah di-log, dan tidak pernah dikirim balik.
	hashed, err := helper.HashPassword(req.Password)
	if err != nil {
		return helper.Internal(err)
	}

	// Role selalu ditentukan server, tidak pernah diambil dari request.
	created, err := s.users.Create(ctx, model.User{
		Username: req.Username,
		Email:    req.Email,
		Password: hashed,
		Role:     "user",
		IsActive: true,
	})
	if err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return helper.Conflict("username sudah dipakai")
		}
		return helper.Internal(err)
	}

	return helper.Created(c, "pendaftaran berhasil", created,
		"/api/v1/users/"+strconv.Itoa(created.ID))
}

// Login godoc
//
// @Summary      Login dan mendapatkan pasangan token
// @Description  Dibatasi 5 kali gagal per menit per IP.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      model.LoginRequest  true  "Username dan password"
// @Success      200   {object}  model.WebResponse{data=model.TokenPair}
// @Failure      401   {object}  model.ErrorResponse  "Username atau password salah"
// @Failure      429   {object}  model.ErrorResponse  "Terlalu banyak percobaan login"
// @Router       /auth/login [post]
func (s *AuthService) Login(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	var req model.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}

	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}

	user, err := s.users.FindByUsername(ctx, strings.TrimSpace(req.Username))
	if err != nil {
		// Username tidak ditemukan. Tetap jalankan pemeriksaan palsu agar
		// waktu tanggapnya mirip dengan kasus password salah, lalu jawab
		// dengan pesan yang SAMA PERSIS. Membedakan keduanya sama saja
		// dengan memberi tahu penyerang username mana yang terdaftar.
		helper.VerifyDummyPassword(req.Password)
		return helper.Unauthorized("username atau password salah")
	}

	if !helper.VerifyPassword(user.Password, req.Password) {
		return helper.Unauthorized("username atau password salah")
	}

	if !user.IsActive {
		return helper.Forbidden("akun dinonaktifkan")
	}

	pair, err := s.issueTokenPair(ctx, user)
	if err != nil {
		return helper.Internal(err)
	}

	return helper.Success(c, fiber.StatusOK, "login berhasil", pair)
}

// Refresh godoc
//
// @Summary      Menukar refresh token dengan pasangan token baru
// @Description  Refresh token lama langsung dicabut (rotation).
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      model.RefreshRequest  true  "Refresh token"
// @Success      200   {object}  model.WebResponse{data=model.TokenPair}
// @Failure      401   {object}  model.ErrorResponse  "Refresh token tidak valid, kedaluwarsa, atau sudah dicabut"
// @Router       /auth/refresh [post]
func (s *AuthService) Refresh(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	var req model.RefreshRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}

	if strings.TrimSpace(req.RefreshToken) == "" {
		return helper.BadRequest("refresh_token wajib diisi")
	}

	hash := helper.SHA256Hex(req.RefreshToken)

	stored, err := s.tokens.FindActive(ctx, hash)
	if err != nil {
		return helper.Unauthorized("refresh token tidak valid atau sudah kedaluwarsa")
	}

	user, err := s.users.FindByID(ctx, stored.UserID)
	if err != nil || !user.IsActive {
		return helper.Unauthorized("akun tidak dapat dipakai")
	}

	// ROTASI: token lama langsung dicabut dan diganti yang baru.
	// Bila token lama sempat dicuri, ia hanya berguna sekali.
	if err := s.tokens.Revoke(ctx, hash); err != nil {
		return helper.Internal(err)
	}

	pair, err := s.issueTokenPair(ctx, user)
	if err != nil {
		return helper.Internal(err)
	}

	return helper.Success(c, fiber.StatusOK, "token berhasil diperbarui", pair)
}

// Logout godoc
//
// @Summary      Logout dan mencabut refresh token
// @Description  Selalu menjawab 200 walaupun token sudah tidak berlaku: dari sudut pandang pemakai, logout harus selalu berhasil.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      model.RefreshRequest  true  "Refresh token yang ingin dicabut"
// @Success      200   {object}  model.WebResponse
// @Failure      400   {object}  model.ErrorResponse  "Body bukan JSON yang valid"
// @Router       /auth/logout [post]
func (s *AuthService) Logout(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	var req model.RefreshRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}

	if strings.TrimSpace(req.RefreshToken) != "" {
		// Kegagalan mencabut tidak dilaporkan sebagai error ke client:
		// dari sudut pandang pemakai, logout harus selalu berhasil.
		_ = s.tokens.Revoke(ctx, helper.SHA256Hex(req.RefreshToken))
	}

	return helper.Success(c, fiber.StatusOK, "logout berhasil", nil)
}

// Me godoc
//
// @Summary      Profil user yang sedang login beserta daftar permission
// @Description  Daftar permission dipakai frontend untuk menampilkan tombol yang sesuai. Ini kemudahan tampilan, bukan pengamanan.
// @Tags         auth
// @Produce      json
// @Success      200  {object}  model.WebResponse{data=model.Profile}
// @Failure      401  {object}  model.ErrorResponse
// @Security     BearerAuth
// @Router       /auth/me [get]
func (s *AuthService) Me(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	authUser, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	user, err := s.users.FindByID(ctx, authUser.UserID)
	if err != nil {
		return helper.Unauthorized("user tidak ditemukan")
	}

	// Modul 6: daftar permission ikut dikirim agar frontend tahu tombol apa
	// yang layak ditampilkan. Ini kemudahan tampilan, BUKAN pengamanan.
	return helper.Success(c, fiber.StatusOK, "profil berhasil diambil", model.Profile{
		User:        user,
		Permissions: s.perms.PermissionsOf(user.Role),
	})
}

// issueTokenPair membuat access token dan refresh token sekaligus.
func (s *AuthService) issueTokenPair(
	ctx context.Context, user model.User,
) (model.TokenPair, error) {
	accessToken, err := s.jwt.GenerateAccess(user)
	if err != nil {
		return model.TokenPair{}, err
	}

	refreshToken, err := helper.RandomToken(refreshTokenBytes)
	if err != nil {
		return model.TokenPair{}, err
	}

	// Yang disimpan hash-nya, bukan tokennya.
	err = s.tokens.Save(ctx, model.RefreshToken{
		UserID:    user.ID,
		TokenHash: helper.SHA256Hex(refreshToken),
		ExpiresAt: time.Now().Add(s.refreshTTL),
	})
	if err != nil {
		return model.TokenPair{}, err
	}

	return model.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(s.jwt.AccessTTL().Seconds()),
	}, nil
}
