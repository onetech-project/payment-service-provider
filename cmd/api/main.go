package main

import (
	"context"

	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"backbone-new/internal/adapter/delivery/http/handler"
	customMiddleware "backbone-new/internal/adapter/delivery/http/middleware"
	"backbone-new/internal/infrastructure/config"
	"backbone-new/internal/infrastructure/crypto"
	"backbone-new/internal/infrastructure/database"
	"backbone-new/internal/infrastructure/redis"
	"backbone-new/internal/infrastructure/telemetry"
	"backbone-new/internal/usecase"

	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"
)

func generateDefaultRSAKeys() (string, string, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}
	privBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privBytes,
	})

	pubBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", "", err
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	})

	return string(privPEM), string(pubPEM), nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}

func main() {
	ctx := context.Background()

	// 1. Initialize Telemetry
	otelEndpoint := getEnvOrDefault("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	shutdownTracer, err := telemetry.InitTracer(ctx, "payment-integration-gateway", otelEndpoint)
	if err != nil {
		log.Printf("Telemetry initialization warning: %v", err)
	} else if shutdownTracer != nil {
		defer func() { _ = shutdownTracer(ctx) }()
	}

	// 2. Database Connection
	dbPort, _ := strconv.Atoi(getEnvOrDefault("DB_PORT", "5432"))
	dbConfig := database.Config{
		Host:     getEnvOrDefault("DB_HOST", "localhost"),
		Port:     dbPort,
		User:     getEnvOrDefault("DB_USER", "postgres"),
		Password: getEnvOrDefault("DB_PASSWORD", "postgres"),
		DBName:   getEnvOrDefault("DB_NAME", "payment_gateway"),
		SSLMode:  getEnvOrDefault("DB_SSLMODE", "disable"),
	}

	pgPool, err := database.NewPostgresPool(ctx, dbConfig)
	if err != nil {
		log.Printf("PostgreSQL connection error: %v (operating with fallback)", err)
	} else {
		defer pgPool.Close()
	}

	// 3. Redis Connection
	redisAddr := getEnvOrDefault("REDIS_ADDR", "localhost:6379")
	redisPassword := getEnvOrDefault("REDIS_PASSWORD", "")
	redisClient, err := redis.NewRedisClient(redisAddr, redisPassword, 0)
	if err != nil {
		log.Fatalf("Fatal: Redis connection required for Idempotency and Queue: %v", err)
	}

	// 4. Crypto & JWT Setup
	privPEM, pubPEM, err := generateDefaultRSAKeys()
	if err != nil {
		log.Fatalf("Fatal: Failed to generate server RSA keys: %v", err)
	}
	jwtIssuer, err := crypto.NewJWTIssuerFromPEM(privPEM, pubPEM)
	if err != nil {
		log.Fatalf("Fatal: JWT issuer setup failed: %v", err)
	}

	rsaVerifier := crypto.NewRSAVerifier()
	var clientRepo *database.ClientRepository
	if pgPool != nil {
		clientRepo = database.NewClientRepository(pgPool)
	}

	// 5. Usecase & Handler Initialization
	tokenUsecase := usecase.NewTokenUsecase(clientRepo, rsaVerifier, jwtIssuer)
	tokenHandler := handler.NewTokenHandler(tokenUsecase)

	// VA Usecase & Handler
	var vaRepo *database.VARepository
	if pgPool != nil {
		vaRepo = database.NewVARepository(pgPool)
	}
	vaUsecase := usecase.NewVAUsecase(vaRepo)
	vaHandler := handler.NewVAHandler(vaUsecase)

	// Load vendor configurations
	configDir := getEnvOrDefault("CONFIG_DIR", ".")
	configLoader := config.NewVendorConfigLoader(configDir)
	vendorConfigs, err := configLoader.LoadAll()
	if err != nil {
		log.Printf("Warning: Failed to load vendor configs: %v", err)
	}

	// 6. Echo Server Setup
	e := echo.New()
	e.HideBanner = true
	e.Use(echoMiddleware.Recover())
	e.Use(customMiddleware.TelemetryMiddleware())

	// Health check endpoint
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"status":  "UP",
			"service": "payment-integration-gateway",
		})
	})

	// SNAP Token Endpoint with Idempotency Middleware
	snapGroup := e.Group("/v1.0")
	snapGroup.Use(customMiddleware.IdempotencyMiddleware(redisClient))
	snapGroup.POST("/access-token/b2b", tokenHandler.GetB2BAccessToken)

	// Register vendor-specific routes
	for _, vc := range vendorConfigs {
		// Create route group for each vendor/channel
		vendorGroup := e.Group("/v1.0/" + vc.Vendor + "/" + vc.Channel)

		// Apply SNAP auth middleware with vendor config
		vendorGroup.Use(customMiddleware.SNAPAuthMiddleware(vc))
		vendorGroup.Use(customMiddleware.IdempotencyMiddleware(redisClient))

		// Register VA endpoints
		vendorGroup.POST("/transfer-va/inquiry", vaHandler.Inquiry)
		vendorGroup.POST("/transfer-va/payment", vaHandler.Payment)
		vendorGroup.POST("/transfer-va/status", vaHandler.Status)

		log.Printf("Registered vendor routes: /v1.0/%s/%s", vc.Vendor, vc.Channel)
	}

	// Default vendor VA routes for backward compatibility
	if len(vendorConfigs) == 0 {
		log.Println("No vendor configs found, using default vendor VA routes")
		vaGroup := e.Group("/v1.0/bca/va")
		vaGroup.Use(customMiddleware.IdempotencyMiddleware(redisClient))
		vaGroup.POST("/inquiry", vaHandler.Inquiry)
		vaGroup.POST("/payment", vaHandler.Payment)
		vaGroup.POST("/status", vaHandler.Status)
	}

	port := getEnvOrDefault("PORT", "8080")
	log.Printf("Starting SNAP Payment Gateway Server on port %s...", port)
	if err := e.Start(fmt.Sprintf(":%s", port)); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
}
