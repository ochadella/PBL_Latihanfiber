package service

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"siakad-mini/app/model"
	"siakad-mini/app/repository"
	"siakad-mini/helper"
)

type CourseService struct {
	courses repository.CourseRepository
}

func NewCourseService(courses repository.CourseRepository) *CourseService {
	return &CourseService{courses: courses}
}

// List -> GET /api/v1/courses?semester=5&search=basis&available=true
func (s *CourseService) List(c *fiber.Ctx) error {
	filter := model.CourseFilter{
		Search: strings.TrimSpace(c.Query("search")),
	}
	errs := helper.ValidationErrors{}

	if v := strings.TrimSpace(c.Query("semester")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 8 {
			errs.Add("semester", "Semester harus berupa angka 1 sampai 8")
		} else {
			filter.Semester = n
		}
	}

	switch strings.ToLower(strings.TrimSpace(c.Query("available"))) {
	case "":
		// tidak difilter
	case "true":
		filter.Available = true
	case "false":
		filter.Available = false
	default:
		errs.Add("available", "Nilai available harus true atau false")
	}

	if errs.HasErrors() {
		return helper.FailValidation(c, errs)
	}

	courses, err := s.courses.FindAll(c.UserContext(), filter)
	if err != nil {
		return err
	}

	return helper.OK(c, "Data mata kuliah berhasil diambil", courses)
}