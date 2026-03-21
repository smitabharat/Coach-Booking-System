package testdb

import (
	"os"
	"testing"

	"github.com/nudgebee/booking-api/internal/database"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// RequireDB opens PostgreSQL using TEST_DATABASE_URL, or falls back to BOOKING_DB_DSN.
// It truncates application tables after each test. Skips if no DSN is set.
func RequireDB(t testing.TB) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("BOOKING_DB_DSN")
	}
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL or BOOKING_DB_DSN to run PostgreSQL integration tests")
	}
	db, err := database.Open(dsn)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Exec(`TRUNCATE TABLE bookings, availabilities, users, coaches RESTART IDENTITY CASCADE`).Error
	})
	return db
}
