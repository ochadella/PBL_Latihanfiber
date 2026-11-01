package service

import (
	"encoding/json"
	"fmt"

	"latihan-fiber/app/model"
)

// JobsForEvent memecah satu baris outbox menjadi task-task yang akan
// dikirim ke queue.
//
// Event KRS menghasilkan satu task. Event course.quota_changed menghasilkan
// satu task PER PESERTA: dengan begitu kegagalan mengirim ke mahasiswa ke-7
// hanya me-retry task mahasiswa itu, bukan mengirim ulang email ke
// mahasiswa ke-1 sampai ke-6.
func JobsForEvent(e model.OutboxEvent) ([]model.NotificationJob, error) {
	switch e.EventType {
	case model.EventEnrollmentCreated,
		model.EventEnrollmentDropped,
		model.EventEnrollmentSwapped:
		var event model.EnrollmentEvent
		if err := json.Unmarshal(e.Payload, &event); err != nil {
			return nil, fmt.Errorf("membaca payload event %d: %w", e.ID, err)
		}
		if event.UserID <= 0 {
			return nil, fmt.Errorf("event %d tidak memiliki user_id", e.ID)
		}
		return []model.NotificationJob{{
			OutboxID: e.ID, UserID: event.UserID,
			EventType: e.EventType, Payload: e.Payload,
		}}, nil

	case model.EventCourseQuotaChanged:
		var event model.QuotaChangedEvent
		if err := json.Unmarshal(e.Payload, &event); err != nil {
			return nil, fmt.Errorf("membaca payload event %d: %w", e.ID, err)
		}
		seen := make(map[int]struct{}, len(event.Recipients))
		jobs := make([]model.NotificationJob, 0, len(event.Recipients))
		for _, userID := range event.Recipients {
			if userID <= 0 {
				continue
			}
			if _, duplicate := seen[userID]; duplicate {
				continue
			}
			seen[userID] = struct{}{}
			jobs = append(jobs, model.NotificationJob{
				OutboxID: e.ID, UserID: userID,
				EventType: e.EventType, Payload: e.Payload,
			})
		}
		return jobs, nil

	default:
		// Event yang tidak dikenal tetap diteruskan sebagai satu task tanpa
		// penerima. Worker akan menolaknya sebagai kegagalan permanen dan
		// mengarsipkannya, bukan membuat relay berhenti selamanya.
		return []model.NotificationJob{{
			OutboxID: e.ID, EventType: e.EventType, Payload: e.Payload,
		}}, nil
	}
}
