package middleware

import (
	"backbone-new/internal/infrastructure/crypto"

	"github.com/labstack/echo/v4"
)

// symmetricSignature is the one place either middleware derives and checks a
// SNAP symmetric X-SIGNATURE:
//
//	HMAC_SHA512(clientSecret, stringToSign)
//	stringToSign = HTTPMethod ":" EndpointUrl ":" AccessToken ":"
//	               Lowercase(HexEncode(SHA-256(Minify(RequestBody)))) ":" Timestamp
//
// It exists because the vendor and merchant sides kept drifting apart on the
// RequestBody component, and every divergence looks identical from outside: a
// bare 401. They disagreed on minification first (vendor minified, merchant
// hashed raw bytes), then on the digest encoding (vendor hex per spec,
// merchant base64). Both were found the expensive way. Sharing the derivation
// means the next change lands on both sides or neither.
//
// What legitimately differs per caller is expressed as data, not as a second
// implementation: which HMAC algorithm, what the AccessToken component holds,
// and which body-hash forms are still accepted during a migration.
type symmetricSignature struct {
	Secret    string
	Algorithm string // empty falls back to HMAC-SHA512, per NewHMACSigner
	Method    string
	// RelativeURL is the EndpointUrl component. SNAP defines it as the path
	// plus query string; every SNAP route here is POST/DELETE without query
	// params, so callers pass URL.Path.
	RelativeURL string
	// AccessToken is the bearer token bound into stringToSign, or "" for
	// callers that have none to bind (legacy vendors without a ClientID).
	AccessToken string
	Timestamp   string
	Body        []byte

	// BodyHashEncodings are the accepted encodings of SHA-256(Minify(body)),
	// canonical first. Anything after the first entry is a transition
	// allowance for signers not yet migrated.
	BodyHashEncodings []string
	// ExtraBodyHashes are already-computed RequestBody components to accept
	// in addition — for digests that are not merely a re-encoding of the
	// canonical one, such as the merchant side's pre-minification digest.
	ExtraBodyHashes []string
}

// verify reports whether provided matches any accepted form of this
// signature. It fails closed: no secret and no candidate digest both mean no.
func (s symmetricSignature) verify(provided string) bool {
	if s.Secret == "" || provided == "" {
		return false
	}

	signer := crypto.NewHMACSigner(s.Secret, s.Algorithm)
	for _, bodyHash := range s.bodyHashCandidates() {
		if signer.Verify(s.stringToSign(bodyHash), provided) {
			return true
		}
	}
	return false
}

// stringToSign assembles the five colon-separated components.
func (s symmetricSignature) stringToSign(bodyHash string) string {
	return crypto.BuildStringToSign(s.Method, s.RelativeURL, s.AccessToken, bodyHash, s.Timestamp)
}

// bodyHashCandidates lists the RequestBody components to try, canonical
// first, with duplicates dropped. De-duplication is what makes a transition
// allowance free rather than a second HMAC per request: for a body that is
// already compact the legacy pre-minification digest is byte-identical to the
// canonical one, so there is nothing extra to verify.
func (s symmetricSignature) bodyHashCandidates() []string {
	candidates := make([]string, 0, len(s.BodyHashEncodings)+len(s.ExtraBodyHashes))
	seen := make(map[string]bool, cap(candidates))

	add := func(hash string) {
		if hash == "" || seen[hash] {
			return
		}
		seen[hash] = true
		candidates = append(candidates, hash)
	}

	for _, encoding := range s.BodyHashEncodings {
		add(crypto.HashRequestBody(s.Body, encoding))
	}
	for _, hash := range s.ExtraBodyHashes {
		add(hash)
	}
	return candidates
}

// signatureFromRequest fills in the components every caller reads straight off
// the request, leaving the credentials and the accepted-encoding policy to the
// caller. bodyBytes is passed in rather than read here because both
// middlewares must buffer the body for the handler anyway.
func signatureFromRequest(c echo.Context, accessToken, timestamp string, bodyBytes []byte) symmetricSignature {
	return symmetricSignature{
		Method:      c.Request().Method,
		RelativeURL: c.Request().URL.Path,
		AccessToken: accessToken,
		Timestamp:   timestamp,
		Body:        bodyBytes,
	}
}
