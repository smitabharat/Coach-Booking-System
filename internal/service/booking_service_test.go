package service_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nudgebee/booking-api/internal/service"
	"github.com/nudgebee/booking-api/internal/testdb"
	"github.com/stretchr/testify/require"
)

func setupDB(t *testing.T) *service.BookingService {
	t.Helper()
	db := testdb.RequireDB(t)
	return service.NewBookingService(db)
}

func TestAvailableSlotsAndBooking(t *testing.T) {
	svc := setupDB(t)
	user, err := svc.CreateUser("Pat", "pat@example.com")
	require.NoError(t, err)
	coach, err := svc.CreateCoach("Alice", "UTC")
	require.NoError(t, err)
	require.NoError(t, svc.SetAvailability(coach.ID, time.Monday, "10:00", "11:00"))

	// 2024-01-01 is a Monday (UTC).
	slots, err := svc.AvailableSlotsUTC(coach.ID, "2024-01-01")
	require.NoError(t, err)
	require.Len(t, slots, 2)
	require.Equal(t, "2024-01-01T10:00:00Z", slots[0].Format(time.RFC3339))
	require.Equal(t, "2024-01-01T10:30:00Z", slots[1].Format(time.RFC3339))

	b, err := svc.BookSlot(user.ID, coach.ID, slots[0])
	require.NoError(t, err)
	require.Equal(t, coach.ID, b.CoachID)

	slots2, err := svc.AvailableSlotsUTC(coach.ID, "2024-01-01")
	require.NoError(t, err)
	require.Len(t, slots2, 1)
	require.True(t, slots2[0].Equal(slots[1]))
}

func TestCancelFreesSlot(t *testing.T) {
	svc := setupDB(t)
	user, err := svc.CreateUser("Sam", "sam@example.com")
	require.NoError(t, err)
	coach, err := svc.CreateCoach("Bob", "UTC")
	require.NoError(t, err)
	require.NoError(t, svc.SetAvailability(coach.ID, time.Tuesday, "09:00", "09:30"))
	// 2024-01-02 is Tuesday.
	slots, err := svc.AvailableSlotsUTC(coach.ID, "2024-01-02")
	require.NoError(t, err)
	require.Len(t, slots, 1)

	b, err := svc.BookSlot(user.ID, coach.ID, slots[0])
	require.NoError(t, err)
	require.NoError(t, svc.CancelBooking(user.ID, b.ID))

	slots2, err := svc.AvailableSlotsUTC(coach.ID, "2024-01-02")
	require.NoError(t, err)
	require.Len(t, slots2, 1)
}

func TestConcurrentBookSameSlot(t *testing.T) {
	svc := setupDB(t)
	user, err := svc.CreateUser("Racer", "racer@example.com")
	require.NoError(t, err)
	coach, err := svc.CreateCoach("Carol", "UTC")
	require.NoError(t, err)
	require.NoError(t, svc.SetAvailability(coach.ID, time.Wednesday, "14:00", "14:30"))
	// 2024-01-03 is Wednesday.
	slots, err := svc.AvailableSlotsUTC(coach.ID, "2024-01-03")
	require.NoError(t, err)
	require.Len(t, slots, 1)
	target := slots[0]

	const n = 40
	var ok atomic.Int32
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_, err := svc.BookSlot(user.ID, coach.ID, target)
			if err == nil {
				ok.Add(1)
			}
		}()
	}
	wg.Wait()
	require.EqualValues(t, 1, ok.Load())
}

func TestCoachTimezoneDateInterpretation(t *testing.T) {
	svc := setupDB(t)
	// America/New_York on 2025-10-28 00:00 local is still 2025-10-27 evening UTC — weekday follows coach-local date.
	coach, err := svc.CreateCoach("Dan", "America/New_York")
	require.NoError(t, err)
	require.NoError(t, svc.SetAvailability(coach.ID, time.Tuesday, "09:00", "09:30"))

	slots, err := svc.AvailableSlotsUTC(coach.ID, "2025-10-28")
	require.NoError(t, err)
	require.NotEmpty(t, slots)
	// Slot is 09:00 Eastern -> 13:00Z during EDT (October is EDT for New York).
	// 2025-10-28 is still Eastern Daylight Time (UTC-4).
	require.Equal(t, "2025-10-28T13:00:00Z", slots[0].Format(time.RFC3339))
}
