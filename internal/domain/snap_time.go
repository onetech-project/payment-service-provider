package domain

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"
)

// SNAPTime is a timestamp field in a SNAP envelope, tolerant of the empty
// string a vendor sends when it has no value to report.
//
// Why this exists: ASPI declares trxDateTime and transactionDate as
// `type: string`, not `format: date-time` (aspi-open-api-va.yaml:282-283), and
// BCA takes that literally. Its 4042601 "Transaction Not Found" response is a
// fully-populated envelope with every field blanked — `"transactionDate":""`
// included — because the shape is fixed even when there is nothing to put in
// it.
//
// Decoded into a plain *time.Time, that blank aborts the whole unmarshal:
// "parsing time \"\" as \"2006-01-02T15:04:05Z07:00\": cannot parse \"\" as
// \"2006\"". The reconciler then reported outcome=error — the vendor could not
// be understood — for what was in fact a clear, useful answer: this VA has no
// payment. Money that was never paid looked identical to a broken integration.
//
// An absent value is represented as the zero Time, which callers already test
// for via the pointer being nil or Time.IsZero.
type SNAPTime struct {
	time.Time
}

// UnmarshalJSON accepts a JSON null, an empty or whitespace-only string, and
// any RFC3339 timestamp. Only a non-empty string that is not a timestamp is an
// error — that is a vendor sending something genuinely unexpected, which
// should still be reported rather than silently dropped.
func (t *SNAPTime) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		t.Time = time.Time{}
		return nil
	}

	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if strings.TrimSpace(raw) == "" {
		t.Time = time.Time{}
		return nil
	}

	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return err
	}
	t.Time = parsed
	return nil
}

// MarshalJSON writes RFC3339, or an empty string for the zero value — the same
// shape BCA uses, so a response this service builds is indistinguishable from
// the vendor's own on this field.
func (t SNAPTime) MarshalJSON() ([]byte, error) {
	if t.Time.IsZero() {
		return []byte(`""`), nil
	}
	return json.Marshal(t.Time.Format(time.RFC3339))
}

// TimePtr returns the instant as *time.Time, or nil when unset, so callers
// that already branch on a nil pointer keep working unchanged.
func (t *SNAPTime) TimePtr() *time.Time {
	if t == nil || t.Time.IsZero() {
		return nil
	}
	out := t.Time
	return &out
}

// NewSNAPTime wraps an instant for a response this service builds. Returns nil
// for a nil or zero input, so an absent timestamp stays absent rather than
// becoming a spurious "0001-01-01".
func NewSNAPTime(t *time.Time) *SNAPTime {
	if t == nil || t.IsZero() {
		return nil
	}
	return &SNAPTime{Time: *t}
}
