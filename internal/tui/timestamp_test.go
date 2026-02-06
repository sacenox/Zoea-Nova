package tui

import (
	"testing"
	"time"
)

func TestFormatTickTimestamp(t *testing.T) {
	tests := []struct {
		name     string
		tick     int64
		ts       time.Time
		expected string
	}{
		{
			name:     "normal_tick_morning",
			tick:     1234,
			ts:       time.Date(2026, 2, 5, 9, 41, 23, 0, time.UTC),
			expected: "T1234 ⬡ [09:41]",
		},
		{
			name:     "normal_tick_afternoon",
			tick:     5678,
			ts:       time.Date(2026, 2, 5, 14, 30, 0, 0, time.UTC),
			expected: "T5678 ⬡ [14:30]",
		},
		{
			name:     "tick_zero",
			tick:     0,
			ts:       time.Date(2026, 2, 5, 0, 0, 0, 0, time.UTC),
			expected: "T0 ⬡ [00:00]",
		},
		{
			name:     "large_tick",
			tick:     999999,
			ts:       time.Date(2026, 2, 5, 23, 59, 0, 0, time.UTC),
			expected: "T999999 ⬡ [23:59]",
		},
		{
			name:     "very_large_tick",
			tick:     12345678,
			ts:       time.Date(2026, 2, 5, 12, 0, 0, 0, time.UTC),
			expected: "T12345678 ⬡ [12:00]",
		},
		{
			name:     "midnight",
			tick:     100,
			ts:       time.Date(2026, 2, 5, 0, 0, 0, 0, time.UTC),
			expected: "T100 ⬡ [00:00]",
		},
		{
			name:     "noon",
			tick:     200,
			ts:       time.Date(2026, 2, 5, 12, 0, 0, 0, time.UTC),
			expected: "T200 ⬡ [12:00]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTickTimestamp(tt.tick, tt.ts)
			if result != tt.expected {
				t.Errorf("formatTickTimestamp(%d, %v) = %q, expected %q",
					tt.tick, tt.ts, result, tt.expected)
			}
		})
	}
}

func TestFormatTickTimestamp_ZeroTime(t *testing.T) {
	// Edge case: zero time should still format reasonably
	tick := int64(42)
	ts := time.Time{}
	result := formatTickTimestamp(tick, ts)
	// Zero time in Go is "0001-01-01 00:00:00 +0000 UTC"
	// When converted to local, it will be whatever the local offset is
	// Just verify format is correct (T## ⬡ [HH:MM])
	localTime := ts.Local()
	expected := "T42 ⬡ [" + localTime.Format("15:04") + "]"
	if result != expected {
		t.Errorf("formatTickTimestamp(%d, zero time) = %q, expected %q",
			tick, result, expected)
	}
}

func TestFormatTickTimestamp_NegativeTick(t *testing.T) {
	// Edge case: negative tick (shouldn't happen in practice but handle gracefully)
	tick := int64(-1)
	ts := time.Date(2026, 2, 5, 10, 30, 0, 0, time.UTC)
	result := formatTickTimestamp(tick, ts)
	expected := "T-1 ⬡ [10:30]"
	if result != expected {
		t.Errorf("formatTickTimestamp(%d, %v) = %q, expected %q",
			tick, ts, result, expected)
	}
}

func TestFormatTickTimestamp_LocalTime(t *testing.T) {
	// The formatter should convert to local time
	tick := int64(500)
	// Create a UTC time
	utcTime := time.Date(2026, 2, 5, 14, 30, 0, 0, time.UTC)

	// Format should convert to local
	result := formatTickTimestamp(tick, utcTime)

	// Convert to local for expected value
	localTime := utcTime.Local()
	expectedTime := localTime.Format("15:04")
	expected := "T500 ⬡ [" + expectedTime + "]"

	if result != expected {
		t.Errorf("formatTickTimestamp(%d, %v) = %q, expected %q",
			tick, utcTime, result, expected)
	}
}
