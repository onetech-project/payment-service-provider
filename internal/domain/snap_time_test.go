package domain

import (
	"encoding/json"
	"testing"
	"time"
)

// The response that exposed this: BCA's 4042601 "Transaction Not Found" is a
// fully-populated envelope with every value blanked. Decoding it must succeed —
// it is a clear answer ("no payment for this VA"), and failing the unmarshal
// reported it as outcome=error instead.
func TestVAStatusResponse_DecodesBCANotFoundEnvelope(t *testing.T) {
	const body = `{"responseCode":"4042601","responseMessage":"Transaction Not Found",` +
		`"virtualAccountData":{"paymentFlagStatus":"","paymentFlagReason":{"indonesia":"","english":""},` +
		`"partnerServiceId":"","customerNo":"","virtualAccountNo":"","inquiryRequestId":"",` +
		`"paymentRequestId":"","paidAmount":{"value":"","currency":""},"paidBills":"",` +
		`"totalAmount":{"value":"","currency":""},"trxDateTime":"","transactionDate":"",` +
		`"referenceNo":"","paymentType":"","flagAdvise":""},"additionalInfo":{}}`

	var resp VAStatusResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("unmarshal BCA not-found envelope: %v", err)
	}
	if resp.ResponseCode != CodeStatusNotFound {
		t.Errorf("responseCode = %q, want %q", resp.ResponseCode, CodeStatusNotFound)
	}
	if resp.VirtualAccountData == nil {
		t.Fatal("virtualAccountData must decode, not be dropped")
	}
	// Blank dates must read as "unset", so settle() falls through to now()
	// rather than writing a zero date into the ledger.
	if got := resp.VirtualAccountData.TransactionDate.TimePtr(); got != nil {
		t.Errorf("TransactionDate.TimePtr() = %v, want nil for a blank value", got)
	}
	if got := resp.VirtualAccountData.TrxDateTime.TimePtr(); got != nil {
		t.Errorf("TrxDateTime.TimePtr() = %v, want nil for a blank value", got)
	}
}

func TestSNAPTime_UnmarshalAcceptsBlankNullAndRFC3339(t *testing.T) {
	for _, tc := range []struct {
		raw       string
		wantUnset bool
	}{
		{`""`, true},
		{`"   "`, true},
		{`null`, true},
		{`"2026-09-17T22:25:22+07:00"`, false},
		{`"2026-09-17T15:25:22Z"`, false},
	} {
		var got SNAPTime
		if err := json.Unmarshal([]byte(tc.raw), &got); err != nil {
			t.Errorf("unmarshal %s: %v", tc.raw, err)
			continue
		}
		if got.Time.IsZero() != tc.wantUnset {
			t.Errorf("unmarshal %s: IsZero() = %v, want %v", tc.raw, got.Time.IsZero(), tc.wantUnset)
		}
	}
}

// A non-empty value that is not a timestamp is still an error: that is a vendor
// sending something genuinely unexpected, which must not be swallowed.
func TestSNAPTime_UnmarshalRejectsGarbage(t *testing.T) {
	var got SNAPTime
	if err := json.Unmarshal([]byte(`"17/09/2026"`), &got); err == nil {
		t.Error("expected an error for a non-RFC3339 timestamp, got nil")
	}
}

// transactionDate is Mandatory on the status response this service serves to
// BCA, so a real value must still render as RFC3339 and never as JSON null.
func TestSNAPTime_MarshalRoundTrip(t *testing.T) {
	instant := time.Date(2026, 9, 17, 22, 25, 22, 0, time.FixedZone("WIB", 7*3600))
	out, err := json.Marshal(NewSNAPTime(&instant))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if want := `"2026-09-17T22:25:22+07:00"`; string(out) != want {
		t.Errorf("marshal = %s, want %s", out, want)
	}

	// The zero value renders as "" — the shape BCA itself uses — not null.
	zero, err := json.Marshal(SNAPTime{})
	if err != nil {
		t.Fatalf("marshal zero: %v", err)
	}
	if string(zero) != `""` {
		t.Errorf("marshal zero = %s, want \"\"", zero)
	}
}

// NewSNAPTime must not turn an absent time into a spurious 0001-01-01.
func TestNewSNAPTime_NilAndZero(t *testing.T) {
	if got := NewSNAPTime(nil); got != nil {
		t.Errorf("NewSNAPTime(nil) = %v, want nil", got)
	}
	var zero time.Time
	if got := NewSNAPTime(&zero); got != nil {
		t.Errorf("NewSNAPTime(zero) = %v, want nil", got)
	}
}
