package snap

import (
	"context"
	"encoding/base64"
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

	client := NewClient(cfg, nil)

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
		signature := r.Header.Get("X-SIGNATURE")
		assert.NotEmpty(t, signature)
		// Feature 012-base64-hash-encoding: outbound signature must be
		// standard base64 (HMAC-SHA256 -> 32 bytes -> 44 chars incl. padding).
		assert.Len(t, signature, 44)
		_, decodeErr := base64.StdEncoding.DecodeString(signature)
		assert.NoError(t, decodeErr, "outbound X-SIGNATURE must be valid standard base64")

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

	client := NewClient(cfg, nil)

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

// A 4xx from a SNAP service is an ANSWER, not a transport failure: 4042601
// "Transaction Not Found" is precisely what a status inquiry for an unpaid VA
// returns, and 4012600 tells operations their credentials are wrong. Both
// carry a responseCode the caller must act on, so the body is handed back
// rather than collapsed into an error string.
func TestClient_Inquiry_4xx_ReturnsTheSNAPBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"responseCode":"4042412","responseMessage":"Invalid Bill/Virtual Account"}`))
	}))
	defer server.Close()

	client := NewClient(errorTestConfig(server.URL), nil)

	resp, err := client.Inquiry(context.Background(), &domain.VAInquiryRequest{
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: "202607221000001234500001",
	})

	assert.NoError(t, err)
	assert.Equal(t, "4042412", resp.ResponseCode)
}

// A 5xx is the vendor failing to answer at all, and stays an error.
func TestClient_Inquiry_5xx_IsAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"responseCode":"5002400"}`))
	}))
	defer server.Close()

	client := NewClient(errorTestConfig(server.URL), nil)

	resp, err := client.Inquiry(context.Background(), &domain.VAInquiryRequest{
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: "202607221000001234500001",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func errorTestConfig(baseURL string) *config.VendorConfig {
	return &config.VendorConfig{
		ClientSecret:       "test-secret",
		SignatureAlgorithm: "HMAC-SHA256",
		RequestTimeout:     30,
		BaseURL:            baseURL,
		ChannelID:          "12345",
		PartnerID:          "partner1",
		APIEndpoints:       map[string]string{"INQUIRY": "/transfer-va/inquiry"},
	}
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

	client := NewClient(cfg, nil)

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
