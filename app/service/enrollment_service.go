package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"siakad-mini/app/model"
	"siakad-mini/app/repository"
	"siakad-mini/helper"
)

type EnrollmentService struct {
	enrollments repository.EnrollmentRepository
}

func NewEnrollmentService(enrollments repository.EnrollmentRepository) *EnrollmentService {
	return &EnrollmentService{enrollments: enrollments}
}

// currentStudentID mencari id mahasiswa milik user yang sedang login.
func (s *EnrollmentService) currentStudentID(c *fiber.Ctx) (int, error) {
	authUser, _ := helper.CurrentUser(c)
	return s.enrollments.FindStudentIDByUserID(c.UserContext(), authUser.ID)
}

// Create -> POST /api/v1/enrollments (mahasiswa)
func (s *EnrollmentService) Create(c *fiber.Ctx) error {
	var req model.CreateEnrollmentRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return helper.FailValidation(c, map[string][]string{
			"body": {"Body harus berupa JSON yang valid (course_id berupa angka)"},
		})
	}
	if errs := ValidateCreateEnrollment(req); errs.HasErrors() {
		return helper.FailValidation(c, errs)
	}

	studentID, err := s.currentStudentID(c)
	if errors.Is(err, repository.ErrStudentNotActive) {
		return helper.Fail(c, fiber.StatusForbidden, "Akun mahasiswa tidak aktif")
	}
	if err != nil {
		return err
	}

	tahun := strings.TrimSpace(*req.TahunAkademik)
	result, err := s.enrollments.Enroll(c.UserContext(), studentID, *req.CourseID, tahun, BatasSKS)

	var sksErr *repository.SKSLimitError
	switch {
	case err == nil:
		return helper.Created(c, "Mata kuliah berhasil ditambahkan ke KRS", result)

	case errors.Is(err, repository.ErrCourseNotFound):
		return helper.FailValidation(c, map[string][]string{
			"course_id": {"Mata kuliah tidak ditemukan"},
		})

	case errors.Is(err, repository.ErrAlreadyEnrolled):
		return helper.Fail(c, fiber.StatusConflict,
			"Mata kuliah ini sudah diambil pada tahun akademik "+tahun)

	case errors.Is(err, repository.ErrQuotaFull):
		return helper.FailValidation(c, map[string][]string{
			"course_id": {"Kuota mata kuliah sudah penuh"},
		})

	case errors.As(err, &sksErr):
		return helper.FailValidation(c, map[string][]string{
			"course_id": {fmt.Sprintf(
				"Total SKS melebihi batas. Batas SKS Anda %d, sudah diambil %d, sisa SKS %d, sedangkan mata kuliah ini %d SKS",
				sksErr.Batas, sksErr.Diambil, sksErr.Sisa(), sksErr.SKSMatkul)},
		})

	case errors.Is(err, repository.ErrStudentNotActive):
		return helper.Fail(c, fiber.StatusForbidden, "Akun mahasiswa tidak aktif")

	default:
		return err
	}
}

// Delete -> DELETE /api/v1/enrollments/{id} (mahasiswa, hanya milik sendiri)
func (s *EnrollmentService) Delete(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id <= 0 {
		return helper.Fail(c, fiber.StatusNotFound, "Data KRS tidak ditemukan")
	}

	studentID, err := s.currentStudentID(c)
	if errors.Is(err, repository.ErrStudentNotActive) {
		return helper.Fail(c, fiber.StatusForbidden, "Akun mahasiswa tidak aktif")
	}
	if err != nil {
		return err
	}

	enrollment, err := s.enrollments.FindByID(c.UserContext(), id)
	if errors.Is(err, repository.ErrNotFound) {
		return helper.Fail(c, fiber.StatusNotFound, "Data KRS tidak ditemukan")
	}
	if err != nil {
		return err
	}

	// Business rule 4: hanya boleh mengubah KRS milik sendiri.
	if enrollment.StudentID != studentID {
		return helper.Fail(c, fiber.StatusForbidden, "Anda tidak boleh membatalkan KRS milik mahasiswa lain")
	}

	// Kuota otomatis bertambah, karena "terisi" dihitung dari jumlah baris enrollments.
	err = s.enrollments.Delete(c.UserContext(), id)
	if errors.Is(err, repository.ErrNotFound) {
		return helper.Fail(c, fiber.StatusNotFound, "Data KRS tidak ditemukan")
	}
	if err != nil {
		return err
	}

	return helper.NoContent(c)
}