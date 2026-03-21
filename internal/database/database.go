package database

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/nudgebee/booking-api/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DefaultLocalDSN connects to database "booking" on localhost:5433 when no env is set (typical dev).
// Override with DATABASE_URL, BOOKING_DB_DSN, or PGPASSWORD + PG* if credentials or port differ.
const DefaultLocalDSN = "postgres://postgres:root@127.0.0.1:5433/booking?sslmode=disable"

// ResolveDSN returns a PostgreSQL URL in this order: DATABASE_URL, BOOKING_DB_DSN,
// DSN built from PGPASSWORD (and optional PGUSER, PGHOST, PGPORT, PGDATABASE), else DefaultLocalDSN.
func ResolveDSN() string {
	if u := os.Getenv("DATABASE_URL"); u != "" {
		return u
	}
	if u := os.Getenv("BOOKING_DB_DSN"); u != "" {
		return u
	}
	if pass := os.Getenv("PGPASSWORD"); pass != "" {
		return composeDSNFromPGVars(pass)
	}
	return DefaultLocalDSN
}

func composeDSNFromPGVars(password string) string {
	user := getenvDefault("PGUSER", "postgres")
	host := getenvDefault("PGHOST", "127.0.0.1")
	port := getenvDefault("PGPORT", "5433")
	db := getenvDefault("PGDATABASE", "booking")
	auth := url.UserPassword(user, password)
	return fmt.Sprintf("postgres://%s@%s/%s?sslmode=disable", auth.String(), net.JoinHostPort(host, port), db)
}

func getenvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// Open returns a GORM PostgreSQL connection with schema and a partial unique index
// so only one active booking exists per (coach_id, slot_start).
// If the target database from the DSN does not exist yet, Open connects to "postgres",
// creates the database, then connects again (password must be correct — unrelated to "database does not exist").
func Open(dsn string) (*gorm.DB, error) {
	cfg := gormConfig()
	db, err := openAndMigrate(dsn, cfg)
	if err == nil {
		return db, nil
	}
	if !isUndefinedDatabase(err) {
		return nil, err
	}
	if err := createDatabaseFromDSN(dsn, cfg); err != nil {
		return nil, fmt.Errorf("create database: %w", err)
	}
	return openAndMigrate(dsn, cfg)
}

func gormConfig() *gorm.Config {
	cfg := &gorm.Config{}
	if os.Getenv("BOOKING_DB_LOG") != "1" {
		cfg.Logger = logger.Default.LogMode(logger.Silent)
	}
	return cfg
}

func openAndMigrate(dsn string, cfg *gorm.Config) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), cfg)
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&models.User{}, &models.Coach{}, &models.Availability{}, &models.Booking{}); err != nil {
		return nil, err
	}
	if err := db.Exec(`
CREATE UNIQUE INDEX IF NOT EXISTS idx_bookings_coach_slot_active
ON bookings (coach_id, slot_start)
WHERE cancelled_at IS NULL
`).Error; err != nil {
		return nil, fmt.Errorf("partial unique index: %w", err)
	}
	return db, nil
}

func isUndefinedDatabase(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "SQLSTATE 3D000") ||
		(strings.Contains(s, "database") && strings.Contains(s, "does not exist"))
}

func createDatabaseFromDSN(dsn string, cfg *gorm.Config) error {
	dbName, adminDSN, err := splitPostgresDSN(dsn)
	if err != nil {
		return err
	}
	if dbName == "" || strings.EqualFold(dbName, "postgres") {
		return fmt.Errorf("cannot auto-create: target database name missing or is postgres")
	}
	adm, err := gorm.Open(postgres.Open(adminDSN), cfg)
	if err != nil {
		return fmt.Errorf("connect to maintenance database: %w", err)
	}
	sqlDB, err := adm.DB()
	if err != nil {
		return err
	}
	// CREATE DATABASE is not transactional; identifier must be safe.
	if !safePGIdent(dbName) {
		return fmt.Errorf("invalid database name %q", dbName)
	}
	_, err = sqlDB.Exec("CREATE DATABASE " + quotePGIdent(dbName))
	if err == nil {
		return nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "already exists") {
		return nil
	}
	return err
}

func splitPostgresDSN(dsn string) (dbName, adminDSN string, err error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", "", fmt.Errorf("parse DSN: %w", err)
	}
	if u.Scheme != "postgres" && u.Scheme != "postgresql" {
		return "", "", fmt.Errorf("auto-create database only supports postgres:// URLs")
	}
	dbName = strings.TrimPrefix(u.Path, "/")
	if dbName == "" {
		return "", "", fmt.Errorf("no database name in DSN path")
	}
	u2 := *u
	u2.Path = "/postgres"
	return dbName, u2.String(), nil
}

func safePGIdent(s string) bool {
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return false
	}
	return len(s) > 0 && len(s) <= 63
}

func quotePGIdent(ident string) string {
	return `"` + strings.ReplaceAll(ident, `"`, `""`) + `"`
}
