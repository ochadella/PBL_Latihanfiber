package service

import (
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"siakad-mini/app/model"
	"siakad-mini/app/repository"
	"siakad-mini/helper"
)

type StudentService struct {
	students repository.StudentRepository
}

func NewStudentService(students repository.StudentRepository) *StudentService {
	return &StudentService{students: students}
}

// parseStudentID membaca {id} dari URL. Bila bukan angka positif, dianggap tidak ditemukan.
func parseStudentID(c *fiber.Ctx) (int, bool) {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

// List -> GET /api/v1/students (admin)
func (s *StudentService) List(c *fiber.Ctx) error {
	errs := helper.ValidationErrors{}
	f := model.StudentFilter{
		Page:    1,
		PerPage: 10,
		Prodi:   strings.TrimSpace(c.Query("prodi")),
		Search:  strings.TrimSpace(c.Query("search")),
		Sort:    strings.TrimSpace(c.Query("sort")),
	}

	if v := c.Query("page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			errs.Add("page", "Page harus berupa angka minimal 1")
		} else {
			f.Page = n
		}
	}
	if v := c.Query("per_page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 50 {
			errs.Add("per_page", "Per_page harus berupa angka 1 sampai 50")
		} else {
			f.PerPage = n
		}
	}
	if v := c.Query("angkatan"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1000 || n > 9999 {
			errs.Add("angkatan", "Angkatan harus berupa 4 digit angka")
		} else {
			f.Angkatan = n
		}
	}
	switch f.Sort {
	case "", "nama", "-nama", "ipk_terakhir", "-ipk_terakhir":
	default:
		errs.Add("sort", "Sort hanya boleh: nama, -nama, ipk_terakhir, -ipk_terakhir")
	}
	if errs.HasErrors() {
		return helper.FailValidation(c, errs)
	}

	students, total, err := s.students.List(c.UserContext(), f)
	if err != nil {
		return err
	}

	lastPage := int(math.Ceil(float64(total) / float64(f.PerPage)))
	if lastPage < 1 {
		lastPage = 1
	}
	return helper.OKList(c, "Data mahasiswa berhasil diambil", students, helper.Meta{
		CurrentPage: f.Page,
		PerPage:     f.PerPage,
		Total:       total,
		LastPage:    lastPage,
	})
}

// Create -> POST /api/v1/students (admin)
func (s *StudentService) Create(c *fiber.Ctx) error {
	var req model.CreateStudentRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return helper.FailValidation(c, map[string][]string{
			"body": {"Body harus berupa JSON yang valid (angkatan dan ipk_terakhir berupa angka)"},
		})
	}

	errs := ValidateCreateStudent(req, time.Now().Year())
	if errs.HasErrors() {
		return helper.FailValidation(c, errs)
	}

	nim := strings.TrimSpace(*req.NIM)
	email := strings.ToLower(strings.TrimSpace(*req.Email))

	// Cek duplikasi dulu supaya pesan error-nya jelas per field.
	ctx := c.UserContext()
	if exists, err := s.students.NIMExists(ctx, nim); err != nil {
		return err
	} else if exists {
		errs.Add("nim", "NIM sudah terdaftar")
	}
	if exists, err := s.students.EmailExists(ctx, email); err != nil {
		return err
	} else if exists {
		errs.Add("email", "Email sudah terdaftar")
	}
	if errs.HasErrors() {
		return helper.FailValidation(c, errs)
	}

	// Password awal = NIM yang di-hash.
	hash, err := helper.HashPassword(nim)
	if err != nil {
		return err
	}

	ipk := 0.0
	if req.IPKTerakhir != nil {
		ipk = *req.IPKTerakhir
	}

	student, err := s.students.Create(ctx, model.NewStudent{
		NIM:          nim,
		Nama:         strings.TrimSpace(*req.Nama),
		Email:        email,
		Prodi:        strings.TrimSpace(*req.Prodi),
		Angkatan:     *req.Angkatan,
		IPKTerakhir:  ipk,
		PasswordHash: hash,
	})
	if errors.Is(err, repository.ErrDuplicate) {
		// Jaga-jaga bila ada 2 request bersamaan dengan NIM/email yang sama.
		return helper.FailValidation(c, map[string][]string{
			"nim": {"NIM atau email sudah terdaftar"},
		})
	}
	if err != nil {
		return err
	}

	return helper.Created(c, "Mahasiswa berhasil ditambahkan", student)
}

// Detail -> GET /api/v1/students/{id} (admin, atau mahasiswa untuk data sendiri)
func (s *StudentService) Detail(c *fiber.Ctx) error {
	id, ok := parseStudentID(c)
	if !ok {
		return helper.Fail(c, fiber.StatusNotFound, "Mahasiswa tidak ditemukan")
	}

	ctx := c.UserContext()
	student, err := s.students.FindByID(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		return helper.Fail(c, fiber.StatusNotFound, "Mahasiswa tidak ditemukan")
	}
	if err != nil {
		return err
	}

	// Mahasiswa hanya boleh melihat datanya sendiri.
	authUser, _ := helper.CurrentUser(c)
	if authUser.Role == model.RoleMahasiswa && student.UserID != authUser.ID {
		return helper.Fail(c, fiber.StatusForbidden, "Anda tidak boleh mengakses data mahasiswa lain")
	}

	courses, err := s.students.FindEnrolledCourses(ctx, student.ID)
	if err != nil {
		return err
	}
	totalSKS := 0
	for _, mk := range courses {
		totalSKS += mk.SKS
	}

	return helper.OK(c, "Detail mahasiswa berhasil diambil", model.StudentDetail{
		Student:    student,
		MataKuliah: courses,
		TotalSKS:   totalSKS,
		BatasSKS:   BatasSKS(student.IPKTerakhir),
	})
}

// Update -> PUT /api/v1/students/{id} (admin)
func (s *StudentService) Update(c *fiber.Ctx) error {
	id, ok := parseStudentID(c)
	if !ok {
		return helper.Fail(c, fiber.StatusNotFound, "Mahasiswa tidak ditemukan")
	}

	var req model.UpdateStudentRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return helper.FailValidation(c, map[string][]string{
			"body": {"Body harus berupa JSON yang valid (angkatan dan ipk_terakhir berupa angka)"},
		})
	}

	if errs := ValidateUpdateStudent(req, time.Now().Year()); errs.HasErrors() {
		return helper.FailValidation(c, errs)
	}

	student, err := s.students.Update(c.UserContext(), id, model.StudentUpdate{
		Nama:        strings.TrimSpace(*req.Nama),
		Prodi:       strings.TrimSpace(*req.Prodi),
		Angkatan:    *req.Angkatan,
		IPKTerakhir: req.IPKTerakhir,
	})
	if errors.Is(err, repository.ErrNotFound) {
		return helper.Fail(c, fiber.StatusNotFound, "Mahasiswa tidak ditemukan")
	}
	if err != nil {
		return err
	}

	return helper.OK(c, "Data mahasiswa berhasil diperbarui", student)
}

// Delete -> DELETE /api/v1/students/{id} (admin, soft delete)
func (s *StudentService) Delete(c *fiber.Ctx) error {
	id, ok := parseStudentID(c)
	if !ok {
		return helper.Fail(c, fiber.StatusNotFound, "Mahasiswa tidak ditemukan")
	}

	err := s.students.SoftDelete(c.UserContext(), id)
	if errors.Is(err, repository.ErrNotFound) {
		return helper.Fail(c, fiber.StatusNotFound, "Mahasiswa tidak ditemukan")
	}
	if err != nil {
		return err
	}

	return helper.NoContent(c)
}