package middleware

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"time"

	"backbone-new/internal/domain"
	"backbone-new/internal/infrastructure/config"

	"github.com/labstack/echo/v4"
)

// maxExternalIDLength is BCA's documented X-EXTERNAL-ID limit (String(36) Max)
// for every transfer-va service.
const maxExternalIDLength = 36

// documentedTransferVAHeaders is the closed set BCA publishes for the
// transfer-va services: the common table in BCA OpenAPI OAuth & Signature
// v1.1 plus the per-service "Additional Headers" tables in
// VA-BillPresentment v2.4, VA-Payment-Flag v2.3 and VA-Payment-Status V2 v1.0.
//
// X-CLIENT-KEY is deliberately absent — it belongs to the access-token
// endpoint alone. So is Idempotency-Key, which appears in no BCA document at
// all; idempotency here is keyed on X-EXTERNAL-ID.
var documentedTransferVAHeaders = map[string]bool{
	"CONTENT-TYPE":                    true,
	"AUTHORIZATION":                   true,
	strings.ToUpper(headerTimestamp):  true,
	strings.ToUpper(headerSignature):  true,
	"ORIGIN":                          true, // Mandatory: N
	strings.ToUpper(headerChannelID):  true,
	strings.ToUpper(headerPartnerID):  true,
	strings.ToUpper(headerExternalID): true,
}

// isDocumentedTransferVAHeader reports whether BCA publishes header for the
// transfer-va services. HTTP header names are case-insensitive, so the
// comparison is too.
func isDocumentedTransferVAHeader(header string) bool {
	return documentedTransferVAHeaders[strings.ToUpper(strings.TrimSpace(header))]
}

// SNAP header names.
const (
	headerTimestamp  = "X-TIMESTAMP"
	headerSignature  = "X-SIGNATURE"
	headerPartnerID  = "X-PARTNER-ID"
	headerExternalID = "X-EXTERNAL-ID"
	headerChannelID  = "CHANNEL-ID"
)

// SNAPAuthMiddleware validates SNAP authentication headers for a single
// vendor. skipSkewCheck disables the ±5 minute X-TIMESTAMP freshness check —
// intended for APP_ENV=dev/uat only.
func SNAPAuthMiddleware(vendorConfig *config.VendorConfig, jwtIssuer domain.JWTIssuer, skipSkewCheck bool) echo.MiddlewareFunc {
	return MultiVendorSNAPAuth([]*config.VendorConfig{vendorConfig}, jwtIssuer, skipSkewCheck)
}

// MultiVendorSNAPAuth authenticates a request against ANY of the configured
// vendors, and records which one on the echo context.
//
// This exists because the router cannot do it. Registering the same
// method+path once per vendor — each wrapped in its own single-vendor
// middleware — does not stack: echo replaces the route, so only the last
// vendor registered was ever reachable and every other vendor's traffic was
// rejected with that vendor's credentials. Resolution has to happen inside one
// handler chain, keyed on what the request actually carries.
//
// Every rejection carries the service code of the endpoint being called
// (24 inquiry / 25 payment / 26 status), because BCA matches responseCode
// against the service it invoked and rejects the transaction outright if the
// code is not one it recognises for that service.
func MultiVendorSNAPAuth(vendorConfigs []*config.VendorConfig, jwtIssuer domain.JWTIssuer, skipSkewCheck bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			service := domain.ServiceCodeForPath(c.Request().URL.Path)

			// The body is read once, here, and re-buffered for the handler
			// (same pattern as IdempotencyMiddleware). Reading it per
			// candidate would consume it after the first attempt.
			bodyBytes, err := io.ReadAll(c.Request().Body)
			if err != nil {
				return snapUnauthorized(c, service, "Unauthorized. [Unable to read request body]")
			}
			c.Request().Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

			candidates := candidateVendors(c, vendorConfigs)
			if len(candidates) == 0 {
				return snapUnauthorized(c, service, "Unauthorized. [Unknown client]")
			}

			// Try each candidate in turn. The first vendor whose credentials
			// verify owns the request; if none does, the first candidate's
			// rejection is the one reported, since that is the vendor the
			// headers claimed to be.
			var firstRejection func(echo.Context) error
			for _, vendorConfig := range candidates {
				rejection := authenticateVendor(c, vendorConfig, jwtIssuer, service, skipSkewCheck, bodyBytes)
				if rejection == nil {
					c.Set(domain.ContextKeyVendor, domain.VendorContext{
						Vendor:                vendorConfig.Vendor,
						Channel:               vendorConfig.Channel,
						StrictMandatoryFields: vendorConfig.StrictMandatoryFields,
					})
					c.Request().Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
					return next(c)
				}
				if firstRejection == nil {
					firstRejection = rejection
				}
			}
			return firstRejection(c)
		}
	}
}

// candidateVendors narrows the configured vendors to those whose pinned
// X-PARTNER-ID / CHANNEL-ID match what the request carries. A vendor that
// pins neither matches anything, which is how a single-vendor deployment with
// no allow-list keeps working.
func candidateVendors(c echo.Context, vendorConfigs []*config.VendorConfig) []*config.VendorConfig {
	partnerID := c.Request().Header.Get(headerPartnerID)
	channelID := c.Request().Header.Get(headerChannelID)

	var candidates []*config.VendorConfig
	for _, vendorConfig := range vendorConfigs {
		if vendorConfig.PartnerID != "" && partnerID != "" && !matchesAllowedValue(partnerID, vendorConfig.PartnerID) {
			continue
		}
		if vendorConfig.ChannelID != "" && channelID != "" && !matchesAllowedValue(channelID, vendorConfig.ChannelID) {
			continue
		}
		candidates = append(candidates, vendorConfig)
	}

	// No vendor claims these header values. Keep every config as a candidate
	// so the rejection comes from the normal header/signature checks — which
	// name the offending header — rather than a bare "unknown client".
	if len(candidates) == 0 {
		return vendorConfigs
	}
	return candidates
}

// authenticateVendor runs the full check chain for one vendor, returning nil
// when the request is authentic and a rejection renderer otherwise.
func authenticateVendor(
	c echo.Context,
	vendorConfig *config.VendorConfig,
	jwtIssuer domain.JWTIssuer,
	service string,
	skipSkewCheck bool,
	bodyBytes []byte,
) func(echo.Context) error {
	if resp := validateSNAPHeaders(c, vendorConfig, service); resp != nil {
		return resp
	}

	timestamp := c.Request().Header.Get(headerTimestamp)
	if resp := validateSNAPTimestamp(timestamp, service, skipSkewCheck); resp != nil {
		return resp
	}

	// Access-token binding (feature 011-vendor-access-token-signature):
	// vendors migrated to ClientID-based onboarding must present a valid
	// `Authorization: Bearer <token>` header, which becomes the AccessToken
	// component of stringToSign below. Legacy vendors (no ClientID
	// configured) keep today's empty-string convention unchanged.
	accessToken, errResp := resolveVendorAccessToken(c, vendorConfig, jwtIssuer, service)
	if errResp != nil {
		return errResp
	}

	// stringToSign per BCA "Signature Symmetric" — derived by
	// symmetricSignature, the same type the merchant side verifies through.
	// Only the credentials and the body-hash encoding are vendor-specific:
	// the encoding stays per-vendor so vendors onboarded under feature
	// 012-base64-hash-encoding keep working, while everyone else gets the
	// spec's hex.
	signature := signatureFromRequest(c, accessToken, timestamp, bodyBytes)
	signature.Secret = vendorConfig.ClientSecret
	signature.Algorithm = vendorConfig.SignatureAlgorithm
	signature.BodyHashEncodings = []string{vendorConfig.BodyHashEncoding}
	if !signature.verify(c.Request().Header.Get(headerSignature)) {
		return snapUnauthorizedFn(service, "Unauthorized. [Signature]")
	}

	return nil
}

// validateSNAPHeaders checks presence, format and — where the vendor config
// pins them — the VALUES of the SNAP headers. Comparing the values is the
// point: a CHANNEL-ID/X-PARTNER-ID that is merely non-empty proves nothing,
// and BCA pins both (CHANNEL-ID 95231 for VA, X-PARTNER-ID = company code).
func validateSNAPHeaders(c echo.Context, vendorConfig *config.VendorConfig, service string) func(echo.Context) error {
	requiredHeaders := vendorConfig.RequiredHeaders
	if len(requiredHeaders) == 0 {
		// Default SNAP required headers per ASPI spec for transfer-va
		// endpoints. X-CLIENT-KEY is NOT part of this list — it is only used
		// on the access-token endpoint, never on transaction endpoints.
		requiredHeaders = []string{"X-TIMESTAMP", "X-SIGNATURE"}
	}

	for _, header := range requiredHeaders {
		header = strings.TrimSpace(header)
		// A vendor config cannot make this service demand a header BCA never
		// sends. The transfer-va header tables are closed sets, so requiring
		// anything outside them (X-CLIENT-KEY being the one that has actually
		// been tried, and Idempotency-Key the one that used to be) would
		// reject every conformant BCA request with a mandatory-field error
		// about a header BCA has no way to supply.
		if !isDocumentedTransferVAHeader(header) {
			continue
		}
		if c.Request().Header.Get(header) == "" {
			return snapMissingMandatory(service, header)
		}
	}

	if resp := validatePinnedHeader(c, headerChannelID, vendorConfig.ChannelID, service, "Unauthorized. [Unknown channel]"); resp != nil {
		return resp
	}
	if resp := validatePinnedHeader(c, headerPartnerID, vendorConfig.PartnerID, service, "Unauthorized. [Unknown client]"); resp != nil {
		return resp
	}

	if c.Request().Method != http.MethodGet {
		externalID := c.Request().Header.Get(headerExternalID)
		if externalID == "" {
			return snapMissingMandatory(service, headerExternalID)
		}
		if len(externalID) > maxExternalIDLength {
			return snapInvalidField(service, headerExternalID)
		}
		// BCA documents X-EXTERNAL-ID as a "Numeric String" on all three
		// transfer-va services, but the wider ASPI standard types it as a
		// plain string (aspi-open-api-va.yaml:57-60). This gateway fronts
		// several vendors, so the narrower rule is applied only to vendors
		// configured for BCA's field tables — the same reason
		// StrictMandatoryFields exists. Enforcing it for everyone would reject
		// legitimate alphanumeric ids from non-BCA vendors.
		if vendorConfig.StrictMandatoryFields && !isNumericString(externalID) {
			return snapInvalidField(service, headerExternalID)
		}
	}

	return nil
}

// isNumericString reports whether s consists solely of ASCII digits.
func isNumericString(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// validatePinnedHeader enforces a header whose accepted value(s) the vendor
// config pins. A blank allowed value means the vendor has not pinned it, in
// which case the header is not enforced here at all.
func validatePinnedHeader(c echo.Context, header, allowed, service, mismatchMessage string) func(echo.Context) error {
	if allowed == "" {
		return nil
	}
	value := c.Request().Header.Get(header)
	if value == "" {
		return snapMissingMandatory(service, header)
	}
	if !matchesAllowedValue(value, allowed) {
		return snapUnauthorizedFn(service, mismatchMessage)
	}
	return nil
}

// matchesAllowedValue reports whether value matches one of a comma-separated
// allow-list. Comma-separated is supported because one deployment commonly
// serves several company codes for the same vendor.
func matchesAllowedValue(value, allowed string) bool {
	for _, entry := range strings.Split(allowed, ",") {
		if strings.TrimSpace(entry) == strings.TrimSpace(value) {
			return true
		}
	}
	return false
}

func validateSNAPTimestamp(timestamp, service string, skipSkewCheck bool) func(echo.Context) error {
	parsed, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return snapInvalidField(service, "X-TIMESTAMP")
	}
	if !skipSkewCheck && (time.Since(parsed) > 5*time.Minute || time.Until(parsed) > 5*time.Minute) {
		return snapUnauthorizedFn(service, "Unauthorized. [Timestamp skew exceeds 5 minutes]")
	}
	return nil
}

// resolveVendorAccessToken implements feature 011-vendor-access-token-signature:
// vendors migrated to ClientID-based onboarding (vendorConfig.ClientID set)
// must present a valid `Authorization: Bearer <token>` header whose ClientID
// claim matches this vendor config; the validated token string is returned
// to be bound into stringToSign. Legacy vendors (ClientID empty) are
// untouched — "", nil is returned immediately, preserving today's
// empty-AccessToken-component convention.
func resolveVendorAccessToken(c echo.Context, vendorConfig *config.VendorConfig, jwtIssuer domain.JWTIssuer, service string) (string, func(echo.Context) error) {
	if vendorConfig.ClientID == "" {
		return "", nil
	}

	authHeader := c.Request().Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return "", snapInvalidTokenFn(service)
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := jwtIssuer.ValidateToken(token)
	if err != nil {
		return "", snapInvalidTokenFn(service)
	}

	if claims.ClientID != vendorConfig.ClientID {
		return "", snapUnauthorizedFn(service, "Unauthorized. [Unknown client]")
	}

	return token, nil
}

// --- rejection helpers -------------------------------------------------
//
// 401s carry the bare two-field body: BCA's Appendix A shows "-" in the
// status column for both Unauthorized and Invalid Token, meaning no
// virtualAccountData is expected. 400s DO carry it, with status "01".

func snapUnauthorized(c echo.Context, service, message string) error {
	return c.JSON(http.StatusUnauthorized,
		domain.NewSNAPErrorResponse(domain.CodeUnauthorized(service), message))
}

func snapUnauthorizedFn(service, message string) func(echo.Context) error {
	return func(c echo.Context) error { return snapUnauthorized(c, service, message) }
}

func snapInvalidTokenFn(service string) func(echo.Context) error {
	return func(c echo.Context) error {
		return c.JSON(http.StatusUnauthorized,
			domain.NewSNAPErrorResponse(domain.CodeInvalidToken(service), "Invalid Token (B2B)"))
	}
}

func snapMissingMandatory(service, field string) func(echo.Context) error {
	return func(c echo.Context) error {
		return c.JSON(http.StatusBadRequest, domain.NewSNAPErrorBody(
			service,
			domain.CodeMissingMandatory(service),
			"Invalid Mandatory Field ["+field+"]",
			domain.VAIdentityEcho{},
		))
	}
}

func snapInvalidField(service, field string) func(echo.Context) error {
	return func(c echo.Context) error {
		return c.JSON(http.StatusBadRequest, domain.NewSNAPErrorBody(
			service,
			domain.CodeInvalidField(service),
			"Invalid Field Format ["+field+"]",
			domain.VAIdentityEcho{},
		))
	}
}
