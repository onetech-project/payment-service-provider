package snap

import (
	"strings"
	"testing"
	"time"
)

// A UTC clock must still produce +07:00, converted — not "Z", and not
// "+00:00". This is the whole reason the helper exists: the timestamp is
// signed into stringToSign, and BCA answers the resulting mismatch with a 504
// Timeout that names nothing.
func TestSnapTimestamp_UTCRendersAsWIB(t *testing.T) {
	got := snapTimestamp(time.Date(2026, 9, 17, 14, 49, 4, 0, time.UTC))
	if want := "2026-09-17T21:49:04+07:00"; got != want {
		t.Errorf("snapTimestamp(UTC) = %q, want %q", got, want)
	}
	if strings.HasSuffix(got, "Z") {
		t.Errorf("snapTimestamp(UTC) = %q, must not end in Z", got)
	}
}

// A host already on Asia/Jakarta is unaffected — the case that worked before
// must keep working byte for byte.
func TestSnapTimestamp_WIBUnchanged(t *testing.T) {
	got := snapTimestamp(time.Date(2026, 9, 17, 21, 49, 4, 0, wib))
	if want := "2026-09-17T21:49:04+07:00"; got != want {
		t.Errorf("snapTimestamp(WIB) = %q, want %q", got, want)
	}
}

// Any other zone is converted too, so the offset is +07:00 for every
// deployment rather than whatever the host happens to be set to.
func TestSnapTimestamp_OtherZonesConverted(t *testing.T) {
	for _, tc := range []struct {
		name string
		loc  *time.Location
		want string
	}{
		// Every row is the same instant, so every row expects the same WIB
		// wall clock — that is the property under test.
		{"WITA (+08)", time.FixedZone("WITA", 8*3600), "2026-09-17T21:49:04+07:00"},
		{"UTC-5", time.FixedZone("EST", -5*3600), "2026-09-17T21:49:04+07:00"},
	} {
		// Same instant in each zone: 14:49:04Z.
		instant := time.Date(2026, 9, 17, 14, 49, 4, 0, time.UTC).In(tc.loc)
		if got := snapTimestamp(instant); got != tc.want {
			t.Errorf("%s: snapTimestamp = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// Converting must not shift the instant — only how it is written.
func TestSnapTimestamp_PreservesInstant(t *testing.T) {
	original := time.Date(2026, 9, 17, 14, 49, 4, 0, time.UTC)
	parsed, err := time.Parse(time.RFC3339, snapTimestamp(original))
	if err != nil {
		t.Fatalf("time.Parse: %v", err)
	}
	if !parsed.Equal(original) {
		t.Errorf("instant shifted: %s != %s", parsed, original)
	}
}

// The output must still parse as RFC3339 — the offset form is a subset of it,
// and the inbound middleware parses vendor timestamps with time.RFC3339.
func TestSnapTimestamp_ParsesAsRFC3339(t *testing.T) {
	for _, loc := range []*time.Location{time.UTC, wib} {
		s := snapTimestamp(time.Date(2026, 9, 17, 14, 49, 4, 0, loc))
		if _, err := time.Parse(time.RFC3339, s); err != nil {
			t.Errorf("time.Parse(RFC3339, %q) = %v", s, err)
		}
	}
}
