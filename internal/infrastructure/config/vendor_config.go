package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// VendorConfig holds generic vendor/channel configuration
type VendorConfig struct {
	// Vendor identification
	Vendor  string
	Channel string

	// Authentication
	ClientID       string
	ClientSecret   string
	PrivateKeyPath string
	PublicKeyPath  string

	// Endpoints
	BaseURL       string
	TokenEndpoint string
	APIEndpoints  map[string]string

	// Channel
	ChannelID    string
	PartnerID    string
	Origin       string

	// Request
	RequestTimeout     int
	SignatureAlgorithm string

	// SNAP Headers (configurable per vendor)
	RequiredHeaders []string

	// Response codes
	ResponseCodeFormat string // e.g., "AAABBCC"

	// Defaults
	Defaults map[string]string

	// Logging
	DebugLogging      bool
	CorrelationHeader string
}

// VendorConfigLoader loads vendor configuration from .env.<vendor>.<channel> files
type VendorConfigLoader struct {
	configDir string
}

// NewVendorConfigLoader creates a new config loader
func NewVendorConfigLoader(configDir string) *VendorConfigLoader {
	return &VendorConfigLoader{configDir: configDir}
}

// Load loads configuration for a specific vendor and channel
func (l *VendorConfigLoader) Load(vendor, channel string) (*VendorConfig, error) {
	config := &VendorConfig{
		Vendor:             vendor,
		Channel:            channel,
		APIEndpoints:       make(map[string]string),
		RequiredHeaders:    []string{"X-TIMESTAMP", "X-CLIENT-KEY", "X-SIGNATURE"},
		ResponseCodeFormat: "AAABBCC",
		Defaults:           make(map[string]string),
		RequestTimeout:     30,
		SignatureAlgorithm: "HMAC-SHA512",
		CorrelationHeader:  "X-Correlation-ID",
	}

	// Try to load from .env.<vendor>.<channel>
	envFile := filepath.Join(l.configDir, ".env."+vendor+"."+channel)
	envVars, err := parseEnvFile(envFile)
	if err == nil {
		l.applyEnvVars(config, envVars)
	}

	// Allow environment variable overrides
	l.applyEnvOverrides(config)

	return config, nil
}

// LoadAll loads all vendor configurations from the config directory
func (l *VendorConfigLoader) LoadAll() ([]*VendorConfig, error) {
	var configs []*VendorConfig

	entries, err := os.ReadDir(l.configDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasPrefix(name, ".env.") {
			continue
		}

		// Parse vendor.channel from filename
		parts := strings.SplitN(name[5:], ".", 2) // Remove ".env." prefix
		if len(parts) != 2 {
			continue
		}

		vendor := parts[0]
		channel := strings.TrimSuffix(parts[1], filepath.Ext(parts[1]))

		config, err := l.Load(vendor, channel)
		if err != nil {
			continue
		}

		configs = append(configs, config)
	}

	return configs, nil
}

func (l *VendorConfigLoader) applyEnvVars(config *VendorConfig, envVars map[string]string) {
	if v, ok := envVars["VENDOR_CLIENT_ID"]; ok {
		config.ClientID = v
	}
	if v, ok := envVars["VENDOR_CLIENT_SECRET"]; ok {
		config.ClientSecret = v
	}
	if v, ok := envVars["VENDOR_PRIVATE_KEY_PATH"]; ok {
		config.PrivateKeyPath = v
	}
	if v, ok := envVars["VENDOR_PUBLIC_KEY_PATH"]; ok {
		config.PublicKeyPath = v
	}
	if v, ok := envVars["VENDOR_BASE_URL"]; ok {
		config.BaseURL = v
	}
	if v, ok := envVars["VENDOR_TOKEN_ENDPOINT"]; ok {
		config.TokenEndpoint = v
	}
	if v, ok := envVars["VENDOR_CHANNEL_ID"]; ok {
		config.ChannelID = v
	}
	if v, ok := envVars["VENDOR_PARTNER_ID"]; ok {
		config.PartnerID = v
	}
	if v, ok := envVars["VENDOR_ORIGIN"]; ok {
		config.Origin = v
	}
	if v, ok := envVars["VENDOR_REQUEST_TIMEOUT"]; ok {
		config.RequestTimeout = parseIntOrDefault(v, 30)
	}
	if v, ok := envVars["VENDOR_SIGNATURE_ALGORITHM"]; ok {
		config.SignatureAlgorithm = v
	}
	if v, ok := envVars["VENDOR_DEBUG_LOGGING"]; ok {
		config.DebugLogging = v == "true"
	}
	if v, ok := envVars["VENDOR_CORRELATION_HEADER"]; ok {
		config.CorrelationHeader = v
	}
	if v, ok := envVars["VENDOR_RESPONSE_CODE_FORMAT"]; ok {
		config.ResponseCodeFormat = v
	}

	// Load required headers (comma-separated)
	if v, ok := envVars["VENDOR_REQUIRED_HEADERS"]; ok {
		config.RequiredHeaders = strings.Split(v, ",")
	}

	// Load API endpoints (ENDPOINT_NAME=url format)
	for key, value := range envVars {
		if strings.HasPrefix(key, "VENDOR_ENDPOINT_") {
			endpointName := strings.TrimPrefix(key, "VENDOR_ENDPOINT_")
			config.APIEndpoints[endpointName] = value
		}
	}
}

func (l *VendorConfigLoader) applyEnvOverrides(config *VendorConfig) {
	if v := os.Getenv("VENDOR_CLIENT_ID"); v != "" {
		config.ClientID = v
	}
	if v := os.Getenv("VENDOR_CLIENT_SECRET"); v != "" {
		config.ClientSecret = v
	}
	if v := os.Getenv("VENDOR_BASE_URL"); v != "" {
		config.BaseURL = v
	}
	if v := os.Getenv("VENDOR_CHANNEL_ID"); v != "" {
		config.ChannelID = v
	}
	if v := os.Getenv("VENDOR_PARTNER_ID"); v != "" {
		config.PartnerID = v
	}
}

// GetConfigPath returns the config file path for a vendor and channel
func GetConfigPath(configDir, vendor, channel string) string {
	return filepath.Join(configDir, ".env."+vendor+"."+channel)
}

func parseIntOrDefault(s string, defaultVal int) int {
	val := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			val = val*10 + int(c-'0')
		} else {
			return defaultVal
		}
	}
	if val == 0 {
		return defaultVal
	}
	return val
}

// parseEnvFile parses a .env file into a map
func parseEnvFile(filename string) (map[string]string, error) {
	vars := make(map[string]string)

	file, err := os.Open(filename)
	if err != nil {
		return vars, err
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Split on first =
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove surrounding quotes if present
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		vars[key] = value
	}

	return vars, scanner.Err()
}
