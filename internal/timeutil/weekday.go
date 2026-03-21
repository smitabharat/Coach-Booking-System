package timeutil

import (
	"fmt"
	"strings"
	"time"
)

// ParseWeekday accepts Go weekday 0-6 or names like "Monday", "tuesday".
func ParseWeekday(s string) (time.Weekday, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, fmt.Errorf("empty weekday")
	}
	// numeric
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err == nil && n >= 0 && n <= 6 {
		return time.Weekday(n), nil
	}
	names := map[string]time.Weekday{
		"sunday":    time.Sunday,
		"monday":    time.Monday,
		"tuesday":   time.Tuesday,
		"wednesday": time.Wednesday,
		"thursday":  time.Thursday,
		"friday":    time.Friday,
		"saturday":  time.Saturday,
	}
	w, ok := names[s]
	if !ok {
		return 0, fmt.Errorf("unknown weekday: %q", s)
	}
	return w, nil
}

// ParseHHMM parses "09:00" or "9:00" as minutes since midnight.
func ParseHHMM(s string) (int, error) {
	s = strings.TrimSpace(s)
	var h, m int
	n, err := fmt.Sscanf(s, "%d:%d", &h, &m)
	if err != nil || n != 2 {
		return 0, fmt.Errorf("invalid time %q (use HH:MM)", s)
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, fmt.Errorf("time out of range: %q", s)
	}
	return h*60 + m, nil
}

var fancyHyphens = strings.NewReplacer(
	"\u2013", "-", "\u2014", "-", "\ufe58", "-", "\u2212", "-",
)

// ParseDateYMDInLocation parses a calendar date in loc.
// Accepts "YYYY-MM-DD", or a datetime string where only the date part is used (e.g. "2026-03-21T00:00:00Z").
func ParseDateYMDInLocation(s string, loc *time.Location) (time.Time, error) {
	s = strings.TrimSpace(fancyHyphens.Replace(s))
	if s == "" {
		return time.Time{}, fmt.Errorf("empty date")
	}
	if idx := strings.IndexByte(s, 'T'); idx >= 0 {
		s = strings.TrimSpace(s[:idx])
	} else if idx := strings.IndexByte(s, ' '); idx >= 0 {
		s = strings.TrimSpace(s[:idx])
	}
	return time.ParseInLocation("2006-01-02", s, loc)
}
