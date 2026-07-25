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
	// Per ASPI spec, X-CLIENT-KEY is only used on the access-token endpoint,
	// never on transfer-va transaction endpoints, so it must not be a default.
	assert.NotContains(t, config.RequiredHeaders, "X-CLIENT-KEY")
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

func TestVendorConfigLoader_Load_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewVendorConfigLoader(tmpDir)

	config, err := loader.Load("nonexistent", "vendor")
	require.NoError(t, err)
	assert.Equal(t, "nonexistent", config.Vendor)
	assert.Equal(t, 30, config.RequestTimeout) // defaults preserved
}

func TestParseIntOrDefault(t *testing.T) {
	assert.Equal(t, 60, parseIntOrDefault("60", 30))
	assert.Equal(t, 30, parseIntOrDefault("not-a-number", 30))
	assert.Equal(t, 30, parseIntOrDefault("0", 30))
	assert.Equal(t, 30, parseIntOrDefault("", 30))
	assert.Equal(t, 30, parseIntOrDefault("12a", 30))
}

func TestParseEnvFile_MalformedLines(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env.test")

	content := `# a comment line

NO_EQUALS_SIGN_HERE
VENDOR_CLIENT_ID="quoted_value"
VENDOR_CLIENT_SECRET='single_quoted'
VENDOR_BASE_URL=unquoted_value
`
	err := os.WriteFile(envFile, []byte(content), 0644)
	require.NoError(t, err)

	vars, err := parseEnvFile(envFile)
	require.NoError(t, err)
	assert.Equal(t, "quoted_value", vars["VENDOR_CLIENT_ID"])
	assert.Equal(t, "single_quoted", vars["VENDOR_CLIENT_SECRET"])
	assert.Equal(t, "unquoted_value", vars["VENDOR_BASE_URL"])
	_, ok := vars["NO_EQUALS_SIGN_HERE"]
	assert.False(t, ok)
}

func TestParseEnvFile_MissingFile(t *testing.T) {
	vars, err := parseEnvFile("/nonexistent/path/.env.foo")
	assert.Error(t, err)
	assert.NotNil(t, vars)
	assert.Len(t, vars, 0)
}

func TestVendorConfigLoader_Load_EnvOverride_ChannelAndPartnerID(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewVendorConfigLoader(tmpDir)

	os.Setenv("VENDOR_CHANNEL_ID", "override-channel")
	os.Setenv("VENDOR_PARTNER_ID", "override-partner")
	os.Setenv("VENDOR_CLIENT_SECRET", "override-secret")
	defer func() {
		os.Unsetenv("VENDOR_CHANNEL_ID")
		os.Unsetenv("VENDOR_PARTNER_ID")
		os.Unsetenv("VENDOR_CLIENT_SECRET")
	}()

	config, err := loader.Load("test", "override")
	require.NoError(t, err)
	assert.Equal(t, "override-channel", config.ChannelID)
	assert.Equal(t, "override-partner", config.PartnerID)
	assert.Equal(t, "override-secret", config.ClientSecret)
}

func TestVendorConfigLoader_LoadAll_SkipsNonEnvAndDirs(t *testing.T) {
	tmpDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tmpDir, ".env.bca.va"), []byte("VENDOR_CLIENT_ID=bca_client"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "not-an-env-file.txt"), []byte("irrelevant"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, ".env.malformed"), []byte("VENDOR_CLIENT_ID=x"), 0644) // no channel part
	require.NoError(t, os.Mkdir(filepath.Join(tmpDir, ".env.subdir"), 0755))

	loader := NewVendorConfigLoader(tmpDir)
	configs, err := loader.LoadAll()
	require.NoError(t, err)
	assert.Len(t, configs, 1)
	assert.Equal(t, "bca", configs[0].Vendor)
}

func TestVendorConfigLoader_LoadAll_DirError(t *testing.T) {
	loader := NewVendorConfigLoader("/nonexistent/config/dir")
	configs, err := loader.LoadAll()
	assert.Error(t, err)
	assert.Nil(t, configs)
}
