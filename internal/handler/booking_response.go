package handler

import (
	"time"

	"github.com/nudgebee/booking-api/internal/models"
)

// bookingUserBrief is nested user info on booking responses (no email, no timestamps).
type bookingUserBrief struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

// bookingCoachBrief is nested coach info on booking responses (no timestamps).
type bookingCoachBrief struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

// bookingOut is the JSON shape for GET/POST booking responses.
type bookingOut struct {
	ID          uint              `json:"id"`
	UserID      uint              `json:"user_id"`
	CoachID     uint              `json:"coach_id"`
	SlotStart   time.Time         `json:"slot_start"`
	CancelledAt *time.Time        `json:"cancelled_at,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	User        bookingUserBrief  `json:"user"`
	Coach       bookingCoachBrief `json:"coach"`
}

func toBookingOut(b *models.Booking) bookingOut {
	return bookingOut{
		ID:          b.ID,
		UserID:      b.UserID,
		CoachID:     b.CoachID,
		SlotStart:   b.SlotStart,
		CancelledAt: b.CancelledAt,
		CreatedAt:   b.CreatedAt,
		User:        bookingUserBrief{ID: b.User.ID, Name: b.User.Name},
		Coach:       bookingCoachBrief{ID: b.Coach.ID, Name: b.Coach.Name},
	}
}

func toBookingOutList(list []models.Booking) []bookingOut {
	if len(list) == 0 {
		return []bookingOut{}
	}
	out := make([]bookingOut, len(list))
	for i := range list {
		out[i] = toBookingOut(&list[i])
	}
	return out
}
