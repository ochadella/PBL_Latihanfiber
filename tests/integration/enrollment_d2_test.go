//go:build integration

// D.2: integration test tambahan untuk POST /enrollments/swap dan
// DELETE /courses/:id/enroll. Keduanya memakai aplikasi lengkap (newAPI)
// dan PostgreSQL sungguhan, sama seperti enroll_api_test.go.
package integration_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"latihan-fiber/app/model"
)

// call mengirim request apa saja atas nama student. body boleh nil.
// Hasilnya status code dan field "code" pada response (kosong bila tidak ada).
func (a api) call(t *testing.T, student model.User, method, path string, body any) (int, string) {
	t.Helper()

	token, err := a.jwt.GenerateAccess(model.User{ID: student.ID, Username: student.Username, Role: "user"})
	if err != nil {
		t.Fatalf("membuat token: %v", err)
	}

	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("menyusun body: %v", err)
		}
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, "/api/v1"+path, reader)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.app.Test(req, -1)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()

	var out struct {
		Code string `json:"code"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out.Code
}

// enrolledCount membaca enrolled_count langsung dari tabel courses.
func enrolledCount(t *testing.T, courseID int) int {
	t.Helper()
	return countRows(t, `SELECT enrolled_count FROM courses WHERE id = $1`, courseID)
}

// isEnrolled menghitung baris enrollments milik student di satu kelas (0 atau 1).
func isEnrolled(t *testing.T, courseID, userID int) int {
	t.Helper()
	return countRows(t, `SELECT COUNT(*) FROM enrollments WHERE course_id = $1 AND user_id = $2`, courseID, userID)
}

// outboxEvents menghitung event dengan jenis tertentu milik student.
func outboxEvents(t *testing.T, userID int, eventType string) int {
	t.Helper()
	return countRows(t,
		`SELECT COUNT(*) FROM outbox WHERE (payload->>'user_id')::INT = $1 AND event_type = $2`,
		userID, eventType)
}

func coursePath(id int, suffix string) string {
	return "/courses/" + strconv.Itoa(id) + suffix
}

// D.2 nomor 1: pindah ke kelas yang sudah penuh harus ditolak SELURUHNYA.
// Mahasiswa tetap di kelas asal, kedua enrolled_count tidak berubah, dan
// tidak ada event enrollment.swapped yang tertinggal di outbox.
func TestSwapEndpoint_TargetFullKeepsEverything(t *testing.T) {
	a := newAPI(t)
	from := seedCourse(t, 3, 40)
	to := seedCourse(t, 3, 1) // kuota SATU kursi
	student := seedStudent(t)
	other := seedStudent(t)

	if status, _ := a.call(t, student, http.MethodPost, coursePath(from.ID, "/enroll"), nil); status != http.StatusCreated {
		t.Fatalf("persiapan: student daftar ke kelas asal, status = %d", status)
	}
	if status, _ := a.call(t, other, http.MethodPost, coursePath(to.ID, "/enroll"), nil); status != http.StatusCreated {
		t.Fatalf("persiapan: mahasiswa lain mengisi kelas tujuan, status = %d", status)
	}

	status, code := a.call(t, student, http.MethodPost, "/enrollments/swap",
		model.SwapRequest{FromCourseID: from.ID, ToCourseID: to.ID})
	if status != http.StatusConflict || code != "COURSE_FULL" {
		t.Fatalf("swap: dapat %d %s, seharusnya 409 COURSE_FULL", status, code)
	}

	if n := isEnrolled(t, from.ID, student.ID); n != 1 {
		t.Errorf("student di kelas asal = %d baris, seharusnya TETAP 1", n)
	}
	if n := isEnrolled(t, to.ID, student.ID); n != 0 {
		t.Errorf("student di kelas tujuan = %d baris, seharusnya 0", n)
	}
	if n := enrolledCount(t, from.ID); n != 1 {
		t.Errorf("enrolled_count kelas asal = %d, seharusnya tetap 1", n)
	}
	if n := enrolledCount(t, to.ID); n != 1 {
		t.Errorf("enrolled_count kelas tujuan = %d, seharusnya tetap 1", n)
	}
	if n := outboxEvents(t, student.ID, model.EventEnrollmentSwapped); n != 0 {
		t.Errorf("event enrollment.swapped = %d, seharusnya 0", n)
	}
}

// D.2 nomor 2: keluar dari kelas yang berhasil menghasilkan 204, menghapus
// baris enrollment, mengurangi enrolled_count tepat satu, dan menulis tepat
// satu event enrollment.dropped.
func TestDropEndpoint_RemovesEnrollment(t *testing.T) {
	a := newAPI(t)
	course := seedCourse(t, 3, 40)
	student := seedStudent(t)

	if status, _ := a.call(t, student, http.MethodPost, coursePath(course.ID, "/enroll"), nil); status != http.StatusCreated {
		t.Fatalf("persiapan: daftar kelas, status = %d", status)
	}
	before := enrolledCount(t, course.ID)

	if status, code := a.call(t, student, http.MethodDelete, coursePath(course.ID, "/enroll"), nil); status != http.StatusNoContent {
		t.Fatalf("drop: dapat %d %s, seharusnya 204", status, code)
	}

	if n := isEnrolled(t, course.ID, student.ID); n != 0 {
		t.Errorf("baris enrollment = %d, seharusnya 0 (sudah dihapus)", n)
	}
	if after := enrolledCount(t, course.ID); after != before-1 {
		t.Errorf("enrolled_count = %d, seharusnya %d (berkurang satu)", after, before-1)
	}
	if n := outboxEvents(t, student.ID, model.EventEnrollmentDropped); n != 1 {
		t.Errorf("event enrollment.dropped = %d, seharusnya tepat 1", n)
	}
}
