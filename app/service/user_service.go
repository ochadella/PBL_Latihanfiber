package service

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"latihan-fiber/app/model"
	"latihan-fiber/app/repository"
	"latihan-fiber/helper"
)

// UserService memegang dua tanggung jawab sekaligus pada struktur baku
// mata kuliah ini: menerima *fiber.Ctx (peran controller) dan menjalankan
// business rules (peran use case).
//
// Modul 7: setiap kegagalan dikembalikan sebagai error (*helper.AppError),
// TIDAK lagi ditulis sendiri sebagai response.
type UserService struct {
	repo  repository.UserRepository
	perms *helper.PermissionSet // Modul 6
}

// NewUserService menerima INTERFACE, bukan struct konkret.
func NewUserService(
	repo repository.UserRepository,
	perms *helper.PermissionSet,
) *UserService {
	return &UserService{repo: repo, perms: perms}
}

// ---------- GET /users ----------
// List godoc
//
// @Summary      Daftar user (admin)
// @Description  Cursor pagination. Kirim meta.next_cursor sebagai parameter cursor untuk halaman berikutnya. Header Accept: text/csv menghasilkan CSV.
// @Tags         users
// @Produce      json
// @Produce      text/csv
// @Param        limit      query     int     false  "Jumlah per halaman"  default(10)
// @Param        search     query     string  false  "Cari berdasarkan username atau email"
// @Param        is_active  query     bool    false  "Saring berdasarkan status aktif"
// @Param        cursor     query     string  false  "Cursor dari meta.next_cursor"
// @Success      200        {object}  model.WebResponse{data=[]model.User,meta=model.CursorMeta}
// @Failure      400        {object}  model.ErrorResponse
// @Failure      403        {object}  model.ErrorResponse  "Butuh permission user:list"
// @Failure      406        {object}  model.ErrorResponse  "Format pada header Accept tidak didukung"
// @Security     BearerAuth
// @Router       /users [get]
func (s *UserService) List(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	// Format dipilih SEBELUM query dijalankan. Bila client meminta format
	// yang tidak dapat kita hasilkan, tidak ada gunanya membebani database
	// untuk hasil yang akan dibuang.
	format, err := helper.Negotiate(c, helper.FormatJSON, helper.FormatCSV)
	if err != nil {
		return err
	}

	q, err := helper.ParseCursorQuery(c)
	if err != nil {
		return err
	}

	rows, err := s.repo.FindAfterCursor(ctx, q)
	if err != nil {
		return helper.Internal(err)
	}

	// Baris tambahan hasil limit+1 dipotong di sini. Ia hanya penanda bahwa
	// masih ada halaman berikutnya, bukan bagian dari halaman ini.
	hasMore := len(rows) > q.Limit
	if hasMore {
		rows = rows[:q.Limit]
	}

	if format == helper.FormatCSV {
		return helper.WriteUsersCSV(c, rows)
	}

	meta := &model.CursorMeta{Limit: q.Limit, HasMore: hasMore}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		meta.NextCursor = helper.EncodeCursor(last.CreatedAt, last.ID)
	}

	return helper.SuccessCursor(c, "daftar user berhasil diambil", rows, meta)
}

// ---------- GET /users/:id ----------
// Get godoc
//
// @Summary      Detail satu user
// @Description  User biasa hanya boleh melihat dirinya sendiri; admin butuh permission user:read:any.
// @Tags         users
// @Produce      json
// @Param        id   path      int  true  "ID user"
// @Success      200  {object}  model.WebResponse{data=model.User}
// @Failure      403  {object}  model.ErrorResponse
// @Failure      404  {object}  model.ErrorResponse
// @Security     BearerAuth
// @Router       /users/{id} [get]
func (s *UserService) Get(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id harus berupa angka positif")
	}

	// Pemeriksaan hak akses dilakukan SEBELUM data diambil.
	if !CanAccessUser(current, id, s.perms, "user:read:any") {
		return helper.Forbidden("tidak berhak mengakses data user lain")
	}

	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return translateError(err, "user")
	}

	return helper.Success(c, fiber.StatusOK, "user ditemukan", user)
}

// ---------- POST /users ----------
// Create godoc
//
// @Summary      Membuat user baru (admin)
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        body  body      model.CreateUserRequest  true  "Data user baru"
// @Success      201   {object}  model.WebResponse{data=model.User}
// @Header       201   {string}  Location  "URL user yang baru dibuat"
// @Failure      403   {object}  model.ErrorResponse  "Butuh permission user:update:any"
// @Failure      409   {object}  model.ErrorResponse  "Username atau email sudah dipakai"
// @Failure      422   {object}  model.ErrorResponse
// @Security     BearerAuth
// @Router       /users [post]
func (s *UserService) Create(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	var req model.CreateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)

	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}

	// Password di-hash sebelum disimpan (Modul 5).
	hashed, err := helper.HashPassword(req.Password)
	if err != nil {
		return helper.Internal(err)
	}

	// Role selalu ditentukan server, tidak pernah diambil dari request.
	newUser, err := s.repo.Create(ctx, model.User{
		Username: req.Username,
		Email:    req.Email,
		Password: hashed,
		Role:     "user",
		IsActive: true,
	})
	if err != nil {
		return translateError(err, "user")
	}

	return helper.Created(c, "user berhasil dibuat", newUser,
		"/api/v1/users/"+strconv.Itoa(newUser.ID))
}

// ---------- PUT /users/:id ----------
// Replace godoc
//
// @Summary      Mengganti seluruh data user (PUT)
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        id    path      int                       true  "ID user"
// @Param        body  body      model.ReplaceUserRequest  true  "Data lengkap pengganti"
// @Success      200   {object}  model.WebResponse{data=model.User}
// @Failure      403   {object}  model.ErrorResponse
// @Failure      404   {object}  model.ErrorResponse
// @Failure      422   {object}  model.ErrorResponse
// @Security     BearerAuth
// @Router       /users/{id} [put]
func (s *UserService) Replace(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id harus berupa angka positif")
	}

	if !CanAccessUser(current, id, s.perms, "user:update:any") {
		return helper.Forbidden("tidak berhak mengubah data user lain")
	}

	var req model.ReplaceUserRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)

	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}

	result, err := s.repo.Update(ctx, model.User{
		ID:       id,
		Username: req.Username,
		Email:    req.Email,
		IsActive: req.IsActive,
	})
	if err != nil {
		return translateError(err, "user")
	}

	return helper.Success(c, fiber.StatusOK, "user berhasil diganti seluruhnya", result)
}

// ---------- PATCH /users/:id ----------
// Patch godoc
//
// @Summary      Mengubah sebagian data user (PATCH)
// @Description  Hanya field yang dikirim yang diubah.
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        id    path      int                     true  "ID user"
// @Param        body  body      model.PatchUserRequest  true  "Field yang ingin diubah"
// @Success      200   {object}  model.WebResponse{data=model.User}
// @Failure      400   {object}  model.ErrorResponse  "Tidak ada field yang dikirim"
// @Failure      403   {object}  model.ErrorResponse
// @Failure      404   {object}  model.ErrorResponse
// @Security     BearerAuth
// @Router       /users/{id} [patch]
func (s *UserService) Patch(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	currentUser, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id harus berupa angka positif")
	}

	if !CanAccessUser(currentUser, id, s.perms, "user:update:any") {
		return helper.Forbidden("tidak berhak mengubah data user lain")
	}

	var req model.PatchUserRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}

	if IsEmptyPatch(req) {
		return helper.BadRequest("tidak ada field yang diubah")
	}

	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}

	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return translateError(err, "user")
	}

	result, err := s.repo.Update(ctx, ApplyPatch(existing, req))
	if err != nil {
		return translateError(err, "user")
	}

	return helper.Success(c, fiber.StatusOK, "user berhasil diperbarui sebagian", result)
}

// ---------- PATCH /users/:id/role ----------
// Dijaga middleware dengan permission role:assign.
// AssignRole godoc
//
// @Summary      Mengubah role user (admin)
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        id    path      int                      true  "ID user"
// @Param        body  body      model.AssignRoleRequest  true  "Role baru"
// @Success      200   {object}  model.WebResponse{data=model.User}
// @Failure      403   {object}  model.ErrorResponse  "Butuh permission role:assign"
// @Failure      404   {object}  model.ErrorResponse
// @Failure      422   {object}  model.ErrorResponse  "Role tidak dikenal"
// @Security     BearerAuth
// @Router       /users/{id}/role [patch]
func (s *UserService) AssignRole(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id harus berupa angka positif")
	}

	var req model.AssignRoleRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}

	if errs := ValidateAssignRole(current, id, req, s.perms); len(errs) > 0 {
		return helper.Validation(errs)
	}

	result, err := s.repo.UpdateRole(ctx, id, strings.TrimSpace(req.Role))
	if err != nil {
		return translateError(err, "user")
	}

	return helper.Success(c, fiber.StatusOK, "role user berhasil diubah", result)
}

// ---------- DELETE /users/:id ----------
// Delete godoc
//
// @Summary   Menghapus user (admin)
// @Tags      users
// @Param     id   path  int  true  "ID user"
// @Success   204  "Berhasil dihapus; tanpa body"
// @Failure   403  {object}  model.ErrorResponse  "Butuh permission user:delete"
// @Failure   404  {object}  model.ErrorResponse
// @Security     BearerAuth
// @Router    /users/{id} [delete]
func (s *UserService) Delete(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id harus berupa angka positif")
	}

	// Punya permission menghapus tidak berarti boleh menghapus dirinya sendiri.
	if current.UserID == id {
		return helper.Forbidden("tidak boleh menghapus akun sendiri")
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return translateError(err, "user")
	}

	return helper.NoContent(c)
}

// translateError mengubah error milik repository menjadi AppError.
//
// Perhatikan tanda tangannya: tidak ada fiber.Ctx. Fungsi ini hanya
// menerjemahkan satu jenis error menjadi jenis lain, dan tidak tahu
// apa pun tentang HTTP. Yang tidak dikenali menjadi Internal — fail
// closed: lebih baik membalas 500 daripada menebak-nebak status.
func translateError(err error, entity string) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return helper.NotFound(entity + " tidak ditemukan")
	case errors.Is(err, repository.ErrDuplicate):
		return helper.Conflict("username sudah dipakai")
	default:
		// PERBAIKAN #6: pada modul tertulis "return nil". Mengembalikan nil
		// berarti "tidak ada error", sehingga kegagalan database berakhir
		// sebagai 200 dengan body kosong — bertentangan dengan komentar di
		// atas yang menyebut fail closed.
		return helper.Internal(err)
	}
}
