package models

import "time"

// User is a client who can book appointments.
type User struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	Name      string `gorm:"size:128;not null" json:"name"`
	Email     string `gorm:"size:255;not null;uniqueIndex" json:"email"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Coach represents a coach account with a home IANA timezone for availability windows.
type Coach struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	Name      string `gorm:"size:128;not null" json:"name"`
	Timezone  string `gorm:"size:64;not null" json:"timezone"` // e.g. America/New_York
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Availability is a weekly recurring window in the coach's local timezone.
type Availability struct {
	ID        uint `gorm:"primaryKey" json:"id"`
	CoachID   uint `gorm:"index;not null" json:"coach_id"`
	Weekday   int  `gorm:"not null" json:"weekday"` // 0=Sunday .. 6=Saturday (Go time.Weekday)
	StartTime string `gorm:"size:5;not null" json:"start_time"` // "HH:MM" 24h local
	EndTime   string `gorm:"size:5;not null" json:"end_time"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Booking is a 30-minute slot reservation. SlotStart is stored in UTC.
type Booking struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	UserID      uint       `gorm:"index;not null" json:"user_id"`
	CoachID     uint       `gorm:"index;not null" json:"coach_id"`
	SlotStart   time.Time  `gorm:"not null" json:"slot_start"`
	CancelledAt *time.Time `json:"cancelled_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	User        User       `gorm:"foreignKey:UserID" json:"-"`
	Coach       Coach      `gorm:"foreignKey:CoachID" json:"-"`
}
