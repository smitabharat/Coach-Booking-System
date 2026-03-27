package service

import (
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/nudgebee/booking-api/internal/models"
	"github.com/nudgebee/booking-api/internal/timeutil"
	"gorm.io/gorm"
)

const slotLen = 30 * time.Minute

// BookingService contains domain logic for availability, slots, and bookings.
type BookingService struct {
	db *gorm.DB
}

func NewBookingService(db *gorm.DB) *BookingService {
	return &BookingService{db: db}
}

// CreateUser registers a user (required before booking).
func (s *BookingService) CreateUser(name, email string) (*models.User, error) {
	name = strings.TrimSpace(name)
	email = strings.TrimSpace(email)
	if name == "" {
		return nil, ErrInvalidUserInput
	}
	addr, err := mail.ParseAddress(email)
	if err != nil {
		return nil, ErrInvalidUserInput
	}
	norm := strings.ToLower(strings.TrimSpace(addr.Address))
	u := &models.User{Name: name, Email: norm}
	if err := s.db.Create(u).Error; err != nil {
		le := strings.ToLower(err.Error())
		if strings.Contains(le, "unique") || strings.Contains(le, "duplicate key") {
			return nil, ErrDuplicateEmail
		}
		return nil, err
	}
	return u, nil
}

// GetUser returns a user by id.
func (s *BookingService) GetUser(id uint) (*models.User, error) {
	var u models.User
	if err := s.db.First(&u, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &u, nil
}

// GetCoach returns a coach by id.
func (s *BookingService) GetCoach(id uint) (*models.Coach, error) {
	var c models.Coach
	if err := s.db.First(&c, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCoachNotFound
		}
		return nil, err
	}
	return &c, nil
}

// CreateCoach registers a coach with an IANA timezone used for interpreting weekly windows.
func (s *BookingService) CreateCoach(name, tz string) (*models.Coach, error) {
	name = strings.TrimSpace(name)
	tz = strings.TrimSpace(tz)
	if name == "" || tz == "" {
		return nil, ErrInvalidCoachInput
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return nil, ErrInvalidTimezone
	}
	c := &models.Coach{Name: name, Timezone: tz}
	if err := s.db.Create(c).Error; err != nil {
		return nil, err
	}
	return c, nil
}

// SetAvailability adds or replaces weekly windows for a coach (same weekday merges by replacing all rows for that weekday optional - spec implies set per call: we append windows; user can add multiple POSTs for same day multiple ranges - OK).
func (s *BookingService) SetAvailability(coachID uint, weekday time.Weekday, startHHMM, endHHMM string) error {
	var coach models.Coach
	if err := s.db.First(&coach, coachID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCoachNotFound
		}
		return err
	}
	sm, err := timeutil.ParseHHMM(startHHMM)
	if err != nil {
		return err
	}
	em, err := timeutil.ParseHHMM(endHHMM)
	if err != nil {
		return err
	}
	if em <= sm {
		return ErrInvalidDayOrWindow
	}
	av := &models.Availability{
		CoachID:   coachID,
		Weekday:   int(weekday),
		StartTime: normalizeHHMM(startHHMM),
		EndTime:   normalizeHHMM(endHHMM),
	}
	return s.db.Create(av).Error
}

func normalizeHHMM(s string) string {
	sm, _ := timeutil.ParseHHMM(s)
	h, m := sm/60, sm%60
	return time.Date(0, 1, 1, h, m, 0, 0, time.UTC).Format("15:04")
}

// AvailableSlotsUTC returns ISO slot starts in UTC for the coach's calendar date (interpreted in the coach's timezone).
func (s *BookingService) AvailableSlotsUTC(coachID uint, dateYMD string) ([]time.Time, error) {
	var coach models.Coach
	if err := s.db.First(&coach, coachID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCoachNotFound
		}
		return nil, err
	}
	loc, err := time.LoadLocation(coach.Timezone)
	if err != nil {
		return nil, ErrInvalidTimezone
	}
	dayStart, err := timeutil.ParseDateYMDInLocation(dateYMD, loc)
	if err != nil {
		return nil, ErrInvalidDate
	}
	dayEnd := dayStart.Add(24 * time.Hour)

	var avs []models.Availability
	if err := s.db.Where("coach_id = ? AND weekday = ?", coachID, int(dayStart.Weekday())).Find(&avs).Error; err != nil {
		return nil, err
	}

	var slots []time.Time
	for _, a := range avs {
		sm, err := timeutil.ParseHHMM(a.StartTime)
		if err != nil {
			continue
		}
		em, err := timeutil.ParseHHMM(a.EndTime)
		if err != nil {
			continue
		}
		y, mo, d := dayStart.Date()
		start := time.Date(y, mo, d, sm/60, sm%60, 0, 0, loc)
		end := time.Date(y, mo, d, em/60, em%60, 0, 0, loc)
		for t := start; t.Add(slotLen).Compare(end) <= 0; t = t.Add(slotLen) {
			slots = append(slots, t.UTC())
		}
	}

	var booked []models.Booking
	if err := s.db.Where("coach_id = ? AND cancelled_at IS NULL AND slot_start >= ? AND slot_start < ?",
		coachID, dayStart.UTC(), dayEnd.UTC()).Find(&booked).Error; err != nil {
		return nil, err
	}
	bookedSet := make(map[time.Time]struct{}, len(booked))
	for _, b := range booked {
		bookedSet[b.SlotStart.UTC().Truncate(time.Minute)] = struct{}{}
	}

	out := make([]time.Time, 0, len(slots))
	for _, st := range slots {
		key := st.UTC().Truncate(time.Minute)
		if _, taken := bookedSet[key]; !taken {
			out = append(out, key)
		}
	}
	return out, nil
}

// BookSlot reserves a slot inside a DB transaction. Concurrent double-book attempts fail on the partial UNIQUE
// (coach_id, slot_start) WHERE cancelled_at IS NULL; you can add SELECT ... FOR UPDATE on the coach row for stricter serialization.
func (s *BookingService) BookSlot(userID, coachID uint, slotUTC time.Time) (*models.Booking, error) {
	slotUTC = slotUTC.UTC().Truncate(time.Minute)
	if slotUTC.Minute()%30 != 0 || slotUTC.Second() != 0 {
		return nil, ErrInvalidSlot
	}

	var created *models.Booking
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var usr models.User
		if err := tx.First(&usr, userID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrUserNotFound
			}
			return err
		}
		var coach models.Coach
		if err := tx.First(&coach, coachID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrCoachNotFound
			}
			return err
		}

		day, err := slotInCoachLocalDate(coach.Timezone, slotUTC)
		if err != nil {
			return err
		}
		slots, err := s.slotsForDayTx(tx, coach, day)
		if err != nil {
			return err
		}
		ok := false
		for _, t0 := range slots {
			if t0.UTC().Truncate(time.Minute).Equal(slotUTC) {
				ok = true
				break
			}
		}
		if !ok {
			return ErrInvalidSlot
		}

		b := &models.Booking{UserID: userID, CoachID: coachID, SlotStart: slotUTC}
		if err := tx.Create(b).Error; err != nil {
			le := strings.ToLower(err.Error())
			if strings.Contains(le, "unique") || strings.Contains(le, "duplicate key") {
				return ErrBookingConflict
			}
			return err
		}
		created = b
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.GetBookingPreloaded(created.ID)
}

// GetBookingPreloaded returns a booking with User and Coach loaded.
func (s *BookingService) GetBookingPreloaded(id uint) (*models.Booking, error) {
	var b models.Booking
	if err := s.db.Preload("User").Preload("Coach").First(&b, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBookingNotFound
		}
		return nil, err
	}
	return &b, nil
}

func slotInCoachLocalDate(tz string, slotUTC time.Time) (string, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return "", ErrInvalidTimezone
	}
	return slotUTC.In(loc).Format("2006-01-02"), nil
}

func (s *BookingService) slotsForDayTx(tx *gorm.DB, coach models.Coach, dateYMD string) ([]time.Time, error) {
	loc, err := time.LoadLocation(coach.Timezone)
	if err != nil {
		return nil, ErrInvalidTimezone
	}
	dayStart, err := timeutil.ParseDateYMDInLocation(dateYMD, loc)
	if err != nil {
		return nil, ErrInvalidDate
	}
	dayEnd := dayStart.Add(24 * time.Hour)

	var avs []models.Availability
	if err := tx.Where("coach_id = ? AND weekday = ?", coach.ID, int(dayStart.Weekday())).Find(&avs).Error; err != nil {
		return nil, err
	}
	var slots []time.Time
	for _, a := range avs {
		sm, err := timeutil.ParseHHMM(a.StartTime)
		if err != nil {
			continue
		}
		em, err := timeutil.ParseHHMM(a.EndTime)
		if err != nil {
			continue
		}
		y, mo, d := dayStart.Date()
		start := time.Date(y, mo, d, sm/60, sm%60, 0, 0, loc)
		end := time.Date(y, mo, d, em/60, em%60, 0, 0, loc)
		for t := start; t.Add(slotLen).Compare(end) <= 0; t = t.Add(slotLen) {
			slots = append(slots, t.UTC().Truncate(time.Minute))
		}
	}
	var booked []models.Booking
	if err := tx.Where("coach_id = ? AND cancelled_at IS NULL AND slot_start >= ? AND slot_start < ?",
		coach.ID, dayStart.UTC(), dayEnd.UTC()).Find(&booked).Error; err != nil {
		return nil, err
	}
	bookedSet := make(map[time.Time]struct{}, len(booked))
	for _, b := range booked {
		bookedSet[b.SlotStart.UTC().Truncate(time.Minute)] = struct{}{}
	}
	out := make([]time.Time, 0, len(slots))
	for _, st := range slots {
		if _, taken := bookedSet[st]; !taken {
			out = append(out, st)
		}
	}
	return out, nil
}

// ListUserBookings returns all non-cancelled bookings for a user with coach details (past and future).
func (s *BookingService) ListUserBookings(userID uint) ([]models.Booking, error) {
	if err := s.db.First(&models.User{}, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	var list []models.Booking
	q := s.db.Where("user_id = ? AND cancelled_at IS NULL", userID).
		Preload("Coach").
		Preload("User").
		Order("slot_start ASC")
	if err := q.Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// CancelBooking marks a booking cancelled if it belongs to the user.
func (s *BookingService) CancelBooking(userID, bookingID uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var usr models.User
		if err := tx.First(&usr, userID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrUserNotFound
			}
			return err
		}
		var b models.Booking
		if err := tx.First(&b, bookingID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrBookingNotFound
			}
			return err
		}
		if b.UserID != userID {
			return ErrForbidden
		}
		if b.CancelledAt != nil {
			return ErrBookingNotFound
		}
		now := time.Now().UTC()
		return tx.Model(&b).Update("cancelled_at", now).Error
	})
}
