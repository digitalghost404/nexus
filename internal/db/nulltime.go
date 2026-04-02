// internal/db/nulltime.go
package db

import (
	"database/sql/driver"
	"fmt"
	"time"
)

// NullTime is a time.Time that is stored as a Unix timestamp in SQLite
// rather than the Go default string format. This allows proper round-tripping
// through the modernc.org/sqlite driver.
type NullTime struct {
	Time  time.Time
	Valid bool
}

// Scan implements sql.Scanner.
func (nt *NullTime) Scan(value interface{}) error {
	if value == nil {
		nt.Time = time.Time{}
		nt.Valid = false
		return nil
	}

	switch v := value.(type) {
	case time.Time:
		nt.Time = v
		nt.Valid = true
	case int64:
		// Unix timestamp stored as INTEGER
		nt.Time = time.Unix(v, 0)
		nt.Valid = true
	case []byte:
		// Try parsing string as Unix timestamp first, then as time string
		if len(v) > 0 {
			var err error
			// First try Unix timestamp
			var t int64
			if _, err = fmt.Sscanf(string(v), "%d", &t); err == nil {
				nt.Time = time.Unix(t, 0)
				nt.Valid = true
			} else {
				// Fall back to parsing as time string (legacy data)
				nt.Time, err = parseNexusTime(string(v))
				nt.Valid = err == nil
			}
		}
	case string:
		if len(v) > 0 {
			var err error
			// First try Unix timestamp
			var t int64
			if _, err = fmt.Sscanf(v, "%d", &t); err == nil {
				nt.Time = time.Unix(t, 0)
				nt.Valid = true
			} else {
				// Fall back to parsing as time string (legacy data)
				nt.Time, err = parseNexusTime(v)
				nt.Valid = err == nil
			}
		}
	default:
		return fmt.Errorf("cannot scan %T into NullTime", value)
	}
	return nil
}

// Value implements driver.Valuer.
func (nt NullTime) Value() (driver.Value, error) {
	if !nt.Valid {
		return nil, nil
	}
	return nt.Time.Unix(), nil
}

// parseNexusTime parses the legacy time format used by nexus: "2026-03-15 16:37:35 -0500 CDT"
func parseNexusTime(s string) (time.Time, error) {
	// Format: "2026-03-15 16:37:35 -0500 CDT"
	// We need to parse the timezone offset and name separately
	return time.Parse("2006-01-02 15:04:05 -0700 MST", s)
}

// ToNullTime converts a *time.Time to NullTime
func ToNullTime(t *time.Time) NullTime {
	if t == nil {
		return NullTime{}
	}
	return NullTime{Time: *t, Valid: true}
}
