package handler

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net/http"
	"strings"

	"backbone-new/internal/domain"

	"github.com/labstack/echo/v4"
)

// ClientHandler exposes admin endpoints to onboard B2B clients: registering
// client_apps and the client_keys used to verify the asymmetric SNAP
// X-SIGNATURE on /openapi/v1.0/access-token/b2b. Mount behind an auth middleware
// (e.g. AdminAuthMiddleware) — this is a trust-anchor management API.
type ClientHandler struct {
	clientUsecase domain.ClientUsecase
}

func NewClientHandler(clientUsecase domain.ClientUsecase) *ClientHandler {
	return &ClientHandler{clientUsecase: clientUsecase}
}

type clientResponse struct {
	Status  string            `json:"status"`
	Message string            `json:"message,omitempty"`
	Client  *domain.ClientApp `json:"client,omitempty"`
	Key     *domain.ClientKey `json:"key,omitempty"`
}

// RegisterClient handles POST /admin/clients: creates a new client_app and,
// if publicKeyPem is supplied, its first client_key in the same call.
// RegisterClient godoc
// @Tags Admin Client Management
// @Summary Register a new B2B client
// @Description Creates a new client_app trust-anchor record and, if publicKeyPem is supplied, its first client_key in the same call. State-changing: creates persistent client onboarding records.
// @Security AdminToken
// @Param X-Admin-API-Key header string true "Static admin API key (ADMIN_API_KEY)"
// @Param request body domain.RegisterClientRequest true "Client registration payload"
// @Success 201 {object} clientResponse
// @Failure 400 {object} clientResponse "Invalid payload, missing clientId/clientName/keyId, or invalid publicKeyPem"
// @Failure 401 {object} map[string]string "Unauthorized. Invalid or missing X-Admin-API-Key header"
// @Failure 500 {object} clientResponse "Internal Server Error"
// @Failure 503 {object} map[string]string "Admin API disabled (ADMIN_API_KEY not set)"
// @Router /admin/clients [post]
func (h *ClientHandler) RegisterClient(c echo.Context) error {
	var req domain.RegisterClientRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, clientResponse{Status: "error", Message: "Invalid request payload"})
	}

	if req.ClientID == "" || req.ClientName == "" {
		return c.JSON(http.StatusBadRequest, clientResponse{Status: "error", Message: "clientId and clientName are required"})
	}

	client := &domain.ClientApp{
		ClientID:   req.ClientID,
		ClientName: req.ClientName,
	}

	var key *domain.ClientKey
	if req.PublicKeyPEM != "" {
		if req.KeyID == "" {
			return c.JSON(http.StatusBadRequest, clientResponse{Status: "error", Message: "keyId is required when publicKeyPem is provided"})
		}
		if err := validateRSAPublicKeyPEM(req.PublicKeyPEM); err != nil {
			return c.JSON(http.StatusBadRequest, clientResponse{Status: "error", Message: "invalid publicKeyPem: " + err.Error()})
		}
		algorithm := strings.TrimSpace(req.Algorithm)
		if algorithm == "" {
			algorithm = "SHA256withRSA"
		}
		key = &domain.ClientKey{
			ClientID:     req.ClientID,
			KeyID:        req.KeyID,
			PublicKeyPEM: req.PublicKeyPEM,
			Algorithm:    algorithm,
		}
	}

	ctx := c.Request().Context()
	if err := h.clientUsecase.RegisterClient(ctx, client, key); err != nil {
		return c.JSON(http.StatusInternalServerError, clientResponse{Status: "error", Message: err.Error()})
	}

	return c.JSON(http.StatusCreated, clientResponse{Status: "ok", Client: client, Key: key})
}

// AddClientKey handles POST /admin/clients/:clientId/keys: registers an
// additional active public key for an existing client (e.g. key rotation).
// AddClientKey godoc
// @Tags Admin Client Management
// @Summary Add a client key
// @Description Registers an additional active public key for an existing client (e.g. key rotation). State-changing: creates a persistent client_key record.
// @Security AdminToken
// @Param X-Admin-API-Key header string true "Static admin API key (ADMIN_API_KEY)"
// @Param clientId path string true "Client identifier"
// @Param request body domain.AddClientKeyRequest true "Client key payload"
// @Success 201 {object} clientResponse
// @Failure 400 {object} clientResponse "Missing clientId, missing keyId/publicKeyPem, or invalid publicKeyPem"
// @Failure 401 {object} map[string]string "Unauthorized. Invalid or missing X-Admin-API-Key header"
// @Failure 500 {object} clientResponse "Internal Server Error"
// @Failure 503 {object} map[string]string "Admin API disabled (ADMIN_API_KEY not set)"
// @Router /admin/clients/{clientId}/keys [post]
func (h *ClientHandler) AddClientKey(c echo.Context) error {
	clientID := c.Param("clientId")
	if clientID == "" {
		return c.JSON(http.StatusBadRequest, clientResponse{Status: "error", Message: "clientId path parameter is required"})
	}

	var req domain.AddClientKeyRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, clientResponse{Status: "error", Message: "Invalid request payload"})
	}

	if req.KeyID == "" || req.PublicKeyPEM == "" {
		return c.JSON(http.StatusBadRequest, clientResponse{Status: "error", Message: "keyId and publicKeyPem are required"})
	}

	if err := validateRSAPublicKeyPEM(req.PublicKeyPEM); err != nil {
		return c.JSON(http.StatusBadRequest, clientResponse{Status: "error", Message: "invalid publicKeyPem: " + err.Error()})
	}

	algorithm := strings.TrimSpace(req.Algorithm)
	if algorithm == "" {
		algorithm = "SHA256withRSA"
	}

	key := &domain.ClientKey{
		ClientID:     clientID,
		KeyID:        req.KeyID,
		PublicKeyPEM: req.PublicKeyPEM,
		Algorithm:    algorithm,
	}

	ctx := c.Request().Context()
	if err := h.clientUsecase.AddClientKey(ctx, key); err != nil {
		return c.JSON(http.StatusInternalServerError, clientResponse{Status: "error", Message: err.Error()})
	}

	return c.JSON(http.StatusCreated, clientResponse{Status: "ok", Key: key})
}

// RevokeClientKey handles DELETE /admin/clients/:clientId/keys/:keyId.
// RevokeClientKey godoc
// @Tags Admin Client Management
// @Summary Revoke a client key
// @Description Revokes (deactivates) a client's public key so it can no longer be used to verify signatures. State-changing: updates a persistent client_key record.
// @Security AdminToken
// @Param X-Admin-API-Key header string true "Static admin API key (ADMIN_API_KEY)"
// @Param clientId path string true "Client identifier"
// @Param keyId path string true "Key identifier to revoke"
// @Success 200 {object} clientResponse
// @Failure 400 {object} clientResponse "Missing clientId or keyId path parameters"
// @Failure 401 {object} map[string]string "Unauthorized. Invalid or missing X-Admin-API-Key header"
// @Failure 500 {object} clientResponse "Internal Server Error"
// @Failure 503 {object} map[string]string "Admin API disabled (ADMIN_API_KEY not set)"
// @Router /admin/clients/{clientId}/keys/{keyId} [delete]
func (h *ClientHandler) RevokeClientKey(c echo.Context) error {
	clientID := c.Param("clientId")
	keyID := c.Param("keyId")
	if clientID == "" || keyID == "" {
		return c.JSON(http.StatusBadRequest, clientResponse{Status: "error", Message: "clientId and keyId path parameters are required"})
	}

	ctx := c.Request().Context()
	if err := h.clientUsecase.RevokeClientKey(ctx, clientID, keyID); err != nil {
		return c.JSON(http.StatusInternalServerError, clientResponse{Status: "error", Message: err.Error()})
	}

	return c.JSON(http.StatusOK, clientResponse{Status: "ok", Message: "key revoked"})
}

// validateRSAPublicKeyPEM ensures the PEM parses as a PKIX-encoded RSA public
// key — the exact format internal/infrastructure/crypto/rsa_verifier.go
// requires. Rejecting bad keys here avoids silently storing a key that would
// always fail signature verification later.
func validateRSAPublicKeyPEM(pemStr string) error {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return errors.New("failed to parse PEM block, expected -----BEGIN PUBLIC KEY-----")
	}

	pubKeyInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return errors.New("not a valid PKIX public key: " + err.Error())
	}

	if _, ok := pubKeyInterface.(*rsa.PublicKey); !ok {
		return errors.New("public key is not RSA")
	}

	return nil
}
