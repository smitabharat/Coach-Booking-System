package timeutil

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseWeekday(t *testing.T) {
	w, err := ParseWeekday("Tuesday")
	require.NoError(t, err)
	require.Equal(t, time.Tuesday, w)

	w2, err := ParseWeekday("3")
	require.NoError(t, err)
	require.Equal(t, time.Wednesday, w2)
}

func TestParseHHMM(t *testing.T) {
	m, err := ParseHHMM("9:05")
	require.NoError(t, err)
	require.Equal(t, 9*60+5, m)
}

func TestParseDateYMDInLocation(t *testing.T) {
	loc := time.UTC
	d, err := ParseDateYMDInLocation("2026-03-21", loc)
	require.NoError(t, err)
	require.Equal(t, 2026, d.Year())
	require.Equal(t, time.March, d.Month())
	require.Equal(t, 21, d.Day())

	d2, err := ParseDateYMDInLocation(" 2026-03-21T00:00:00Z ", loc)
	require.NoError(t, err)
	require.True(t, d2.Equal(d))
}
