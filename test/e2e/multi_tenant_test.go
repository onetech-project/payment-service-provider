package e2e

import (
	"net/http"
	"testing"

	"backbone-new/internal/domain"
	"backbone-new/internal/infrastructure/config"
	"backbone-new/internal/infrastructure/crypto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This service fronts several vendors (banks/switchers) for several merchants
// (billers) at once. The BCA-conformance work touched shared code — signature
// computation, header enforcement, field strictness — so these tests pin the
// property that none of it leaked across tenant boundaries: a vendor's own
// credentials, header values, hash encoding and field rules apply to that
// vendor only, and a merchant's VAs are reachable only through its own
// partnerServiceId.

func vendorBCA() vendor {
	cfg := defaultVendorConfig()
	cfg.Vendor, cfg.Channel = "bca", "va"
	cfg.ClientID = "bca-client"
	cfg.ClientSecret = "bca-secret"
	cfg.ChannelID = "95231"
	cfg.PartnerID = "12345"
	return vendor{name: "bca", config: cfg}
}

// vendorLegacy models a second vendor onboarded under feature
// 012-base64-hash-encoding, with its own secret, channel and a laxer field
// table. It is also left un-migrated for feature
// 011-vendor-access-token-signature — no ClientID — so the legacy
// empty-AccessToken convention stays exercised end to end alongside BCA's
// token-bound one.
func vendorLegacy() vendor {
	cfg := defaultVendorConfig()
	cfg.Vendor, cfg.Channel = "legacy", "va"
	cfg.ClientID = ""
	cfg.ClientSecret = "legacy-secret"
	cfg.ChannelID = "77001"
	cfg.PartnerID = "67890"
	cfg.BodyHashEncoding = crypto.BodyHashBase64
	cfg.StrictMandatoryFields = false
	return vendor{name: "legacy", config: cfg}
}

// callAs signs with a specific vendor's credentials and headers, including the
// accessToken bound into that vendor's stringToSign.
func (s *server) callAs(t *testing.T, v vendor, path string, payload any, extra ...func(*requestOptions)) response {
	t.Helper()
	opts := []func(*requestOptions){
		withSecret(v.config.ClientSecret),
		withChannelID(v.config.ChannelID),
		withPartnerID(v.config.PartnerID),
		withBodyEncoding(v.config.BodyHashEncoding),
		withClientID(v.config.ClientID),
	}
	return s.call(t, path, payload, append(opts, extra...)...)
}

func TestE2E_MultiVendor_EachVendorUsesItsOwnCredentials(t *testing.T) {
	bca, legacy := vendorBCA(), vendorLegacy()
	s := newServer(t, bca, legacy)
	seedFixedBill(s, padPartnerServiceID(bca.config.PartnerID), "678901234567890300", "1000.00")
	seedFixedBill(s, padPartnerServiceID(legacy.config.PartnerID), "678901234567890301", "2000.00")

	// Each vendor's own credentials work.
	fromBCA := s.callAs(t, bca, paymentPath,
		paymentPayload(padPartnerServiceID(bca.config.PartnerID), "678901234567890300", "PAY-MV-1", "1000.00"))
	require.Equal(t, http.StatusOK, fromBCA.status, fromBCA.raw)

	fromLegacy := s.callAs(t, legacy, paymentPath,
		paymentPayload(padPartnerServiceID(legacy.config.PartnerID), "678901234567890301", "PAY-MV-2", "2000.00"))
	require.Equal(t, http.StatusOK, fromLegacy.status, fromLegacy.raw)
}

func TestE2E_MultiVendor_CredentialsDoNotCrossOver(t *testing.T) {
	bca, legacy := vendorBCA(), vendorLegacy()
	s := newServer(t, bca, legacy)
	seedFixedBill(s, padPartnerServiceID(bca.config.PartnerID), "678901234567890302", "1000.00")

	// BCA's partner/channel headers with the legacy vendor's secret and
	// encoding: neither vendor config can validate this.
	resp := s.call(t, paymentPath,
		paymentPayload(padPartnerServiceID(bca.config.PartnerID), "678901234567890302", "PAY-MV-3", "1000.00"),
		withChannelID(bca.config.ChannelID),
		withPartnerID(bca.config.PartnerID),
		withSecret(legacy.config.ClientSecret),
		withBodyEncoding(legacy.config.BodyHashEncoding))

	assert.Equal(t, http.StatusUnauthorized, resp.status, resp.raw)
	assert.Equal(t, "4012500", resp.code())
}

func TestE2E_MultiVendor_PartnerAndChannelPinnedPerVendor(t *testing.T) {
	bca, legacy := vendorBCA(), vendorLegacy()
	s := newServer(t, bca, legacy)
	seedFixedBill(s, padPartnerServiceID(bca.config.PartnerID), "678901234567890303", "1000.00")

	// Legacy's CHANNEL-ID presented with BCA's secret: the value check is
	// what catches it, and it used to be absent entirely — the header was
	// only tested for non-emptiness.
	resp := s.call(t, paymentPath,
		paymentPayload(padPartnerServiceID(bca.config.PartnerID), "678901234567890303", "PAY-MV-4", "1000.00"),
		withChannelID(legacy.config.ChannelID),
		withPartnerID(bca.config.PartnerID),
		withSecret(bca.config.ClientSecret))

	assert.Equal(t, http.StatusUnauthorized, resp.status, resp.raw)
}

func TestE2E_MultiVendor_BodyHashEncodingIsPerVendor(t *testing.T) {
	// The conformance fix moved the default body-hash encoding to hex. A
	// vendor still signing with base64 must keep working, and must not be
	// able to authenticate against the hex-configured vendor.
	bca, legacy := vendorBCA(), vendorLegacy()
	s := newServer(t, bca, legacy)
	seedNoBillAccount(s, padPartnerServiceID(legacy.config.PartnerID), "678901234567890304")

	// base64 against the base64 vendor: accepted.
	ok := s.callAs(t, legacy, paymentPath,
		paymentPayload(padPartnerServiceID(legacy.config.PartnerID), "678901234567890304", "PAY-ENC-OK", "5000.00"))
	require.Equal(t, http.StatusOK, ok.status, ok.raw)

	// hex against the same vendor: rejected, because the encoding is pinned
	// rather than tried both ways.
	bad := s.callAs(t, legacy, paymentPath,
		paymentPayload(padPartnerServiceID(legacy.config.PartnerID), "678901234567890304", "PAY-ENC-BAD", "5000.00"),
		withBodyEncoding(crypto.BodyHashHex))
	assert.Equal(t, http.StatusUnauthorized, bad.status, bad.raw)
}

func TestE2E_MultiVendor_FieldStrictnessIsPerVendor(t *testing.T) {
	// BCA's extended mandatory set must not be forced onto a vendor that
	// legitimately omits those fields — that would break the other tenants
	// the moment BCA conformance was turned on.
	bca, legacy := vendorBCA(), vendorLegacy()
	s := newServer(t, bca, legacy)
	seedFixedBill(s, padPartnerServiceID(bca.config.PartnerID), "678901234567890305", "1000.00")
	seedFixedBill(s, padPartnerServiceID(legacy.config.PartnerID), "678901234567890306", "1000.00")

	sparse := func(partnerServiceID, customerNo, paymentRequestID string) map[string]any {
		p := paymentPayload(partnerServiceID, customerNo, paymentRequestID, "1000.00")
		delete(p, "flagAdvise")
		delete(p, "trxDateTime")
		delete(p, "channelCode")
		delete(p, "virtualAccountName")
		return p
	}

	// Strict vendor rejects it...
	strict := s.callAs(t, bca, paymentPath, sparse(padPartnerServiceID(bca.config.PartnerID), "678901234567890305", "PAY-STRICT-1"))
	assert.Equal(t, http.StatusBadRequest, strict.status, strict.raw)
	assert.Equal(t, "4002502", strict.code())

	// ...while the lax vendor accepts the same shape.
	lax := s.callAs(t, legacy, paymentPath, sparse(padPartnerServiceID(legacy.config.PartnerID), "678901234567890306", "PAY-LAX-1"))
	require.Equal(t, http.StatusOK, lax.status, lax.raw)
	assert.Equal(t, domain.CodePaymentSuccess, lax.code())
}

func TestE2E_MultiMerchant_VAsAreIsolatedByPartnerServiceID(t *testing.T) {
	// Two merchants under the same vendor, each with its own partnerServiceId
	// and its own bill amount. A payment must land on the merchant that owns
	// the VA number, and the amount check must use THAT merchant's bill.
	bca := vendorBCA()
	bca.config.PartnerID = "12345,67890" // one vendor serving two company codes
	s := newServer(t, bca)

	seedFixedBill(s, padPartnerServiceID("12345"), "678901234567890310", "100000.00")
	seedFixedBill(s, padPartnerServiceID("67890"), "678901234567890311", "250000.00")

	merchantA := s.callAs(t, bca, paymentPath,
		paymentPayload(padPartnerServiceID("12345"), "678901234567890310", "PAY-MM-1", "100000.00"),
		withPartnerID("12345"))
	require.Equal(t, http.StatusOK, merchantA.status, merchantA.raw)

	merchantB := s.callAs(t, bca, paymentPath,
		paymentPayload(padPartnerServiceID("67890"), "678901234567890311", "PAY-MM-2", "250000.00"),
		withPartnerID("67890"))
	require.Equal(t, http.StatusOK, merchantB.status, merchantB.raw)

	// Merchant A's amount against a still-unpaid VA of merchant B is rejected
	// on B's bill, not A's.
	seedFixedBill(s, padPartnerServiceID("67890"), "678901234567890312", "250000.00")
	crossed := s.callAs(t, bca, paymentPath,
		paymentPayload(padPartnerServiceID("67890"), "678901234567890312", "PAY-MM-3", "100000.00"),
		withPartnerID("67890"))
	assert.Equal(t, http.StatusNotFound, crossed.status, crossed.raw)
	assert.Equal(t, domain.CodePaymentInvalidAmt, crossed.code())
}

func TestE2E_MultiMerchant_UnknownVAUnderKnownPartnerRejected(t *testing.T) {
	// A valid vendor and a valid partnerServiceId still cannot pay a VA
	// number no merchant issued.
	bca := vendorBCA()
	s := newServer(t, bca)
	seedFixedBill(s, padPartnerServiceID(bca.config.PartnerID), "678901234567890312", "1000.00")

	resp := s.callAs(t, bca, paymentPath,
		paymentPayload(padPartnerServiceID(bca.config.PartnerID), "678901234567890399", "PAY-MM-4", "1000.00"))

	assert.Equal(t, http.StatusNotFound, resp.status, resp.raw)
	assert.Equal(t, domain.CodePaymentNotFound, resp.code())
	assert.Equal(t, 0, s.notifier.count())
}

func TestE2E_MultiMerchant_CallbacksGoToTheOwningMerchant(t *testing.T) {
	bca := vendorBCA()
	bca.config.PartnerID = "12345,67890"
	s := newServer(t, bca)

	a := seedFixedBill(s, padPartnerServiceID("12345"), "678901234567890320", "100000.00")
	a.NotificationURL = "https://merchant-a.example/callback"
	b := seedFixedBill(s, padPartnerServiceID("67890"), "678901234567890321", "250000.00")
	b.NotificationURL = "https://merchant-b.example/callback"

	require.Equal(t, http.StatusOK, s.callAs(t, bca, paymentPath,
		paymentPayload(padPartnerServiceID("12345"), "678901234567890320", "PAY-CB-1", "100000.00"),
		withPartnerID("12345")).status)
	require.Equal(t, http.StatusOK, s.callAs(t, bca, paymentPath,
		paymentPayload(padPartnerServiceID("67890"), "678901234567890321", "PAY-CB-2", "250000.00"),
		withPartnerID("67890")).status)

	require.Equal(t, 2, s.notifier.count())
	urls := map[string]string{}
	for _, p := range s.notifier.payloads {
		urls[p.VirtualAccountNo] = p.NotificationURL
	}
	// The VA number keeps partnerServiceId's left padding — it is documented as
	// "partnerServiceId (8 digit left padding space “ “) + customerNo" — so the
	// callback reports the 26-character form, not a trimmed one.
	assert.Equal(t, "https://merchant-a.example/callback",
		urls[padPartnerServiceID("12345")+"678901234567890320"])
	assert.Equal(t, "https://merchant-b.example/callback",
		urls[padPartnerServiceID("67890")+"678901234567890321"])
}

func TestE2E_MultiVendor_CommaSeparatedPartnerAllowList(t *testing.T) {
	// One deployment commonly serves several company codes for the same
	// vendor, so the allow-list is comma-separated.
	cfg := defaultVendorConfig()
	cfg.PartnerID = "12345, 67890 ,11111"
	s := newServer(t, vendor{name: "bca", config: cfg})
	seedFixedBill(s, padPartnerServiceID("11111"), "678901234567890330", "1000.00")

	accepted := s.call(t, paymentPath,
		paymentPayload(padPartnerServiceID("11111"), "678901234567890330", "PAY-ALLOW-1", "1000.00"),
		withPartnerID("11111"))
	require.Equal(t, http.StatusOK, accepted.status, accepted.raw)

	rejected := s.call(t, paymentPath,
		paymentPayload(padPartnerServiceID("11111"), "678901234567890330", "PAY-ALLOW-2", "1000.00"),
		withPartnerID("22222"))
	assert.Equal(t, http.StatusUnauthorized, rejected.status, rejected.raw)
}

func TestE2E_VendorConfigDefaultsAreBCAConformant(t *testing.T) {
	// A freshly loaded vendor config, with no .env pinning anything, must
	// default to the spec: hex body hash and the full mandatory field set.
	// A vendor opts OUT explicitly; it never opts in by accident.
	loader := config.NewVendorConfigLoader(t.TempDir())
	cfg, err := loader.Load("newbank", "va")

	require.NoError(t, err)
	assert.Equal(t, crypto.BodyHashHex, cfg.BodyHashEncoding)
	assert.True(t, cfg.StrictMandatoryFields)
	assert.Equal(t, "HMAC-SHA512", cfg.SignatureAlgorithm)
}
