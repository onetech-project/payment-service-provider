package crypto

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BCA's Signature Symmetric section defines the RequestBody component of
// stringToSign as Lowercase(HexEncode(SHA-256(MinifyJson(RequestBody)))).
// These tests pin both halves of that: the minify step and the encoding.

func TestHashRequestBody_DefaultsToLowercaseHex(t *testing.T) {
	body := []byte(`{"partnerServiceId":"   12345"}`)
	sum := sha256.Sum256(body)

	// An unset encoding must degrade toward the spec, not away from it.
	got := HashRequestBody(body, "")

	assert.Equal(t, hex.EncodeToString(sum[:]), got)
	assert.Equal(t, got, HashRequestBody(body, BodyHashHex))
	assert.NotContains(t, got, "=", "hex output is never base64-padded")
}

func TestHashRequestBody_Base64ForMigratedVendors(t *testing.T) {
	// Vendors onboarded under feature 012-base64-hash-encoding sign with
	// base64 and must keep working.
	body := []byte(`{"partnerServiceId":"   12345"}`)
	sum := sha256.Sum256(body)

	got := HashRequestBody(body, BodyHashBase64)

	assert.Equal(t, base64.StdEncoding.EncodeToString(sum[:]), got)
	assert.NotEqual(t, HashRequestBody(body, BodyHashHex), got)
}

func TestHashRequestBody_UnknownEncodingFallsBackToHex(t *testing.T) {
	body := []byte(`{"a":1}`)

	assert.Equal(t, HashRequestBody(body, BodyHashHex), HashRequestBody(body, "b64"))
}

func TestHashRequestBody_MinifiesBeforeHashing(t *testing.T) {
	// BCA: "Before hashed with SHA-256, RequestBody must to convert to
	// MinifyJSON". A pretty-printed body and its minified form must produce
	// the same hash, or a vendor sending indented JSON can never match.
	pretty := []byte("{\n  \"partnerServiceId\": \"   12345\",\n  \"customerNo\": \"123\"\n}")
	minified := []byte(`{"partnerServiceId":"   12345","customerNo":"123"}`)

	assert.Equal(t, HashRequestBody(minified, BodyHashHex), HashRequestBody(pretty, BodyHashHex))
}

func TestHashRequestBody_PreservesWhitespaceInsideValues(t *testing.T) {
	// "remove whitespace except for the key or value json" — the leading
	// spaces in a padded partnerServiceId are DATA and must survive minifying.
	withPadding := []byte(`{"partnerServiceId":"   12345"}`)
	withoutPadding := []byte(`{"partnerServiceId":"12345"}`)

	assert.NotEqual(t,
		HashRequestBody(withPadding, BodyHashHex),
		HashRequestBody(withoutPadding, BodyHashHex))
}

func TestMinifyJSON_LeavesNonJSONUnchanged(t *testing.T) {
	// A malformed body is about to be rejected on its own terms; minifying
	// must not turn that into a different error class.
	malformed := []byte(`{not json`)

	assert.Equal(t, malformed, MinifyJSON(malformed))
	assert.Equal(t, []byte(nil), MinifyJSON(nil))
}

func TestHashRequestBody_EmptyBody(t *testing.T) {
	// "If the RequestBody is empty, set it to empty string" — the hash of the
	// empty string, not a skipped component.
	emptySHA256 := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	assert.Equal(t, emptySHA256, HashRequestBody(nil, BodyHashHex))
	assert.Equal(t, emptySHA256, HashRequestBody([]byte(""), BodyHashHex))
}

func TestBuildStringToSign_ColonSeparated(t *testing.T) {
	got := BuildStringToSign("POST", "/openapi/v1.0/transfer-va/payment", "tok", "abc123", "2026-08-06T10:05:00+07:00")

	assert.Equal(t, "POST:/openapi/v1.0/transfer-va/payment:tok:abc123:2026-08-06T10:05:00+07:00", got)
}

func TestHMACSigner_VerifyAcceptsUnpaddedBase64(t *testing.T) {
	signer := NewHMACSigner("secret", "HMAC-SHA512")
	stringToSign := "POST:/x::abc:2026-08-06T10:05:00+07:00"

	padded := signer.Sign(stringToSign)
	raw, err := base64.StdEncoding.DecodeString(padded)
	require.NoError(t, err)
	unpadded := base64.RawStdEncoding.EncodeToString(raw)

	assert.True(t, signer.Verify(stringToSign, padded))
	assert.True(t, signer.Verify(stringToSign, unpadded), "some SNAP clients omit base64 padding")
	assert.False(t, signer.Verify(stringToSign, "not-a-signature"))
}
