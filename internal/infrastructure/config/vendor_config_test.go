package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVendorConfigLoader_Load_Defaults(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewVendorConfigLoader(tmpDir)

	config, err := loader.Load("testvendor", "testchannel")

	require.NoError(t, err)
	assert.Equal(t, "testvendor", config.Vendor)
	assert.Equal(t, "testchannel", config.Channel)
	assert.Equal(t, 30, config.RequestTimeout)
	assert.Equal(t, "HMAC-SHA512", config.SignatureAlgorithm)
	assert.Equal(t, "X-Correlation-ID", config.CorrelationHeader)
	assert.NotNil(t, config.RequiredHeaders)
	assert.Contains(t, config.RequiredHeaders, "X-TIMESTAMP")
}

func TestVendorConfigLoader_Load_FromFile(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env.bca.va")

	content := `VENDOR_CLIENT_ID=test_client_id
VENDOR_CLIENT_SECRET=test_secret
VENDOR_BASE_URL=https://sandbox.bca.co.id
VENDOR_CHANNEL_ID=95231
VENDOR_PARTNER_ID=12345
VENDOR_REQUEST_TIMEOUT=60
VENDOR_ENDPOINT_INQUIRY=/openapi/v1.0/transfer-va/inquiry
VENDOR_REQUIRED_HEADERS=X-TIMESTAMP,X-CLIENT-KEY,X-SIGNATURE
`
	err := os.WriteFile(envFile, []byte(content), 0644)
	require.NoError(t, err)

	loader := NewVendorConfigLoader(tmpDir)
	config, err := loader.Load("bca", "va")

	require.NoError(t, err)
	assert.Equal(t, "bca", config.Vendor)
	assert.Equal(t, "va", config.Channel)
	assert.Equal(t, "test_client_id", config.ClientID)
	assert.Equal(t, "test_secret", config.ClientSecret)
	assert.Equal(t, "https://sandbox.bca.co.id", config.BaseURL)
	assert.Equal(t, "95231", config.ChannelID)
	assert.Equal(t, "12345", config.PartnerID)
	assert.Equal(t, 60, config.RequestTimeout)
	assert.Equal(t, "/openapi/v1.0/transfer-va/inquiry", config.APIEndpoints["INQUIRY"])
	assert.Equal(t, []string{"X-TIMESTAMP", "X-CLIENT-KEY", "X-SIGNATURE"}, config.RequiredHeaders)
}

func TestVendorConfigLoader_Load_EnvOverride(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env.test.vendor")

	content := `VENDOR_CLIENT_ID=file_client_id
VENDOR_BASE_URL=https://file.example.com
`
	err := os.WriteFile(envFile, []byte(content), 0644)
	require.NoError(t, err)

	os.Setenv("VENDOR_CLIENT_ID", "env_client_id")
	defer os.Unsetenv("VENDOR_CLIENT_ID")

	loader := NewVendorConfigLoader(tmpDir)
	config, err := loader.Load("test", "vendor")

	require.NoError(t, err)
	assert.Equal(t, "env_client_id", config.ClientID) // Env should override file
	assert.Equal(t, "https://file.example.com", config.BaseURL)
}

func TestVendorConfigLoader_LoadAll(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple vendor configs
	content1 := `VENDOR_CLIENT_ID=bca_client`
	content2 := `VENDOR_CLIENT_ID=mandiri_client`

	_ = os.WriteFile(filepath.Join(tmpDir, ".env.bca.va"), []byte(content1), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, ".env.mandiri.va"), []byte(content2), 0644)

	loader := NewVendorConfigLoader(tmpDir)
	configs, err := loader.LoadAll()

	require.NoError(t, err)
	assert.Len(t, configs, 2)

	// Verify both configs loaded
	vendors := make(map[string]string)
	for _, cfg := range configs {
		vendors[cfg.Vendor] = cfg.ClientID
	}
	assert.Equal(t, "bca_client", vendors["bca"])
	assert.Equal(t, "mandiri_client", vendors["mandiri"])
}

func TestGetConfigPath(t *testing.T) {
	path := GetConfigPath("/config", "bca", "va")
	assert.Equal(t, "/config/.env.bca.va", path)
}
