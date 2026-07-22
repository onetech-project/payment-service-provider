package snap

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"backbone-new/internal/domain"
	"backbone-new/internal/infrastructure/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	cfg := &config.VendorConfig{
		ClientSecret:       "test-secret",
		SignatureAlgorithm: "HMAC-SHA256",
		RequestTimeout:     30,
		BaseURL:            "https://api.test.com",
		ChannelID:          "12345",
		PartnerID:          "partner1",
	}

	client := NewClient(cfg)

	assert.NotNil(t, client)
	assert.NotNil(t, client.hmacSigner)
	assert.NotNil(t, client.httpClient)
}

func TestClient_Inquiry_Success(t *testing.T) {
	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "/transfer-va/inquiry")
		assert.NotEmpty(t, r.Header.Get("X-TIMESTAMP"))
		assert.NotEmpty(t, r.Header.Get("X-SIGNATURE"))

		response := domain.VAInquiryResponse{
			ResponseCode:    "2002400",
			ResponseMessage: "Successful",
			VirtualAccountData: &domain.VAAccountData{
				InquiryStatus: "00",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := &config.VendorConfig{
		ClientSecret:       "test-secret",
		SignatureAlgorithm: "HMAC-SHA256",
		RequestTimeout:     30,
		BaseURL:            server.URL,
		ChannelID:          "12345",
		PartnerID:          "partner1",
		APIEndpoints:       map[string]string{"INQUIRY": "/transfer-va/inquiry"},
	}

	client := NewClient(cfg)

	req := &domain.VAInquiryRequest{
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: "202607221000001234500001",
	}

	resp, err := client.Inquiry(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, "2002400", resp.ResponseCode)
	assert.NotNil(t, resp.VirtualAccountData)
}

func TestClient_Inquiry_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "bad request"}`))
	}))
	defer server.Close()

	cfg := &config.VendorConfig{
		ClientSecret:       "test-secret",
		SignatureAlgorithm: "HMAC-SHA256",
		RequestTimeout:     30,
		BaseURL:            server.URL,
		ChannelID:          "12345",
		PartnerID:          "partner1",
		APIEndpoints:       map[string]string{"INQUIRY": "/transfer-va/inquiry"},
	}

	client := NewClient(cfg)

	req := &domain.VAInquiryRequest{
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: "202607221000001234500001",
	}

	resp, err := client.Inquiry(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestClient_PaymentStatus_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := domain.VAStatusResponse{
			ResponseCode:    "2002600",
			ResponseMessage: "Successful",
			VirtualAccountData: &domain.VAStatusData{
				PaymentFlagStatus: "00",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := &config.VendorConfig{
		ClientSecret:       "test-secret",
		SignatureAlgorithm: "HMAC-SHA256",
		RequestTimeout:     30,
		BaseURL:            server.URL,
		ChannelID:          "12345",
		PartnerID:          "partner1",
		APIEndpoints:       map[string]string{"STATUS": "/transfer-va/status"},
	}

	client := NewClient(cfg)

	req := &domain.VAStatusRequest{
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: "202607221000001234500001",
	}

	resp, err := client.PaymentStatus(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, "2002600", resp.ResponseCode)
}
