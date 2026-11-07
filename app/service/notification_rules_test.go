package service

import (
	"errors"
	"strings"
	"testing"

	"latihan-fiber/app/model"
)

func TestBuildMessage(t *testing.T) {
	user := model.User{ID: 7, Username: "mhsuji", Email: "mhsuji@student.unair.ac.id"}

	// CATATAN: modul memakai model.EnrollmentEvent. Pada Tugas Mandiri D.2
	// Modul 11, BuildMessage diubah agar menerima model.NotificationEvent
	// (gabungan field event KRS dan event perubahan kuota), jadi test ini
	// memakai tipe tersebut. Isi field-nya sama persis dengan di modul.
	event := model.NotificationEvent{
		UserID: 7, CourseID: 4, CourseCode: "IF302", CourseName: "Keamanan Informasi",
		FromCourseCode: "IF102", FromCourseName: "Basis Data Lanjut",
		// CATATAN T3: field kuota ditambahkan agar case course.quota_changed
		// (tambahan Modul 11) ikut teruji dan BuildMessage mencapai 100%.
		OldQuota: 40, NewQuota: 45,
	}

	tests := []struct {
		eventType   string
		wantSubject string
	}{
		{model.EventEnrollmentCreated, "KRS: berhasil terdaftar di IF302 Keamanan Informasi"},
		{model.EventEnrollmentDropped, "KRS: keluar dari IF302 Keamanan Informasi"},
		{model.EventEnrollmentSwapped, "KRS: pindah dari IF102 Basis Data Lanjut ke IF302 Keamanan Informasi"},
		{model.EventCourseQuotaChanged, "KRS: kuota IF302 Keamanan Informasi berubah menjadi 45"},
	}

	for _, tt := range tests {
		t.Run(tt.eventType, func(t *testing.T) {
			msg, err := BuildMessage(tt.eventType, event, user)
			if err != nil {
				t.Fatalf("error tidak diharapkan: %v", err)
			}
			if msg.Subject != tt.wantSubject {
				t.Errorf("subject = %q, seharusnya %q", msg.Subject, tt.wantSubject)
			}
			if msg.To != user.Email {
				t.Errorf("to = %q, seharusnya %q", msg.To, user.Email)
			}
			if !strings.Contains(msg.Body, user.Username) {
				t.Errorf("body tidak menyebut username: %q", msg.Body)
			}
		})
	}

	t.Run("event tidak dikenal", func(t *testing.T) {
		_, err := BuildMessage("enrollment.unknown", event, user)
		if !errors.Is(err, errUnknownEvent) {
			t.Errorf("error = %v, seharusnya membungkus errUnknownEvent", err)
		}
	})
}
