package service

import "errors"

var (
	ErrCoachNotFound      = errors.New("coach not found")
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidUserInput   = errors.New("invalid user name or email")
	ErrDuplicateEmail     = errors.New("email already registered")
	ErrInvalidCoachInput  = errors.New("invalid coach name or timezone")
	ErrInvalidDate        = errors.New("invalid date format (use YYYY-MM-DD)")
	ErrInvalidSlot        = errors.New("slot is not available or not on a 30-minute boundary")
	ErrBookingConflict    = errors.New("slot already booked")
	ErrBookingNotFound    = errors.New("booking not found")
	ErrForbidden          = errors.New("forbidden")
	ErrInvalidTimezone    = errors.New("invalid IANA timezone")
	ErrInvalidDayOrWindow = errors.New("invalid day or time window")
)
