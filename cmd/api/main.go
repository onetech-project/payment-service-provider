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
	"time"

	"backbone-new/internal/adapter/delivery/http/handler"
	customMiddleware "backbone-new/internal/adapter/delivery/http/middleware"
	"backbone-new/internal/adapter/delivery/worker"
	"backbone-new/internal/domain"
	"backbone-new/internal/infrastructure/config"
	"backbone-new/internal/infrastructure/crypto"
	"backbone-new/internal/infrastructure/database"
	"backbone-new/internal/infrastructure/queue"
	"backbone-new/internal/infrastructure/redis"
	"backbone-new/internal/infrastructure/telemetry"
	"backbone-new/internal/usecase"

	"github.com/hibiken/asynq"
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

// getEnvDurationSeconds reads an integer-seconds env var into a time.Duration,
// falling back to defaultSeconds when unset or invalid.
func getEnvDurationSeconds(key string, defaultSeconds int) time.Duration {
	seconds := defaultSeconds
	if val := os.Getenv(key); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			seconds = parsed
		}
	}
	return time.Duration(seconds) * time.Second
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
	redisConnectTimeout := getEnvDurationSeconds("REDIS_CONNECT_TIMEOUT_SECONDS", 1)
	redisClient, err := redis.NewRedisClient(redisAddr, redisPassword, 0, redisConnectTimeout)
	if err != nil {
		log.Fatalf("Fatal: Redis connection required for Idempotency and Queue: %v", err)
	}

	// Idempotency TTLs are operational tuning knobs, not constants: lockTTL
	// bounds how long a duplicate concurrent request is held off while the
	// original is in flight; cacheTTL is how long a completed response is
	// replayed for a repeated Idempotency-Key. Default cacheTTL is 24h — a
	// previous hardcoded 1s effectively made replay useless beyond the same
	// instant.
	idempotencyLockTTL := getEnvDurationSeconds("IDEMPOTENCY_LOCK_TTL_SECONDS", 30)
	idempotencyCacheTTL := getEnvDurationSeconds("IDEMPOTENCY_CACHE_TTL_SECONDS", 86400)

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
	rsaSigner := crypto.NewRSASigner()
	var clientRepo *database.ClientRepository
	if pgPool != nil {
		clientRepo = database.NewClientRepository(pgPool)
	}

	// 5. Usecase & Handler Initialization
	tokenUsecase := usecase.NewTokenUsecase(clientRepo, rsaVerifier, jwtIssuer)
	tokenHandler := handler.NewTokenHandler(tokenUsecase)

	signatureUsecase := usecase.NewSignatureUsecase(rsaSigner)
	signatureHandler := handler.NewSignatureHandler(signatureUsecase)

	// Client onboarding (admin) Usecase & Handler
	clientKeyCache := redis.NewClientKeyCache(redisClient)
	clientUsecase := usecase.NewClientUsecase(clientRepo, clientKeyCache)
	clientHandler := handler.NewClientHandler(clientUsecase)
	adminAPIKey := getEnvOrDefault("ADMIN_API_KEY", "")

	// Asynq Client for async notifications
	asynqClient, err := queue.NewClient(redisAddr, redisPassword, 0)
	if err != nil {
		log.Printf("Warning: Asynq client initialization failed: %v", err)
	} else {
		defer func() { _ = asynqClient.Close() }()
	}

	// VA Usecase & Handler
	var vaRepo *database.VARepository
	if pgPool != nil {
		vaRepo = database.NewVARepository(pgPool)
	}
	var notifier domain.NotificationEnqueuer
	if asynqClient != nil {
		notifier = asynqClient
	}
	vaUsecase := usecase.NewVAUsecase(vaRepo, notifier)
	vaHandler := handler.NewVAHandler(vaUsecase)

	// Merchant VA Usecase & Handler
	merchantVAUsecase := usecase.NewMerchantVAUsecase(vaRepo)
	merchantVAHandler := handler.NewMerchantVAHandler(merchantVAUsecase)

	// Asynq Worker for payment notifications
	notificationSecret := getEnvOrDefault("NOTIFICATION_SECRET", "default-secret")
	paymentWorker := worker.NewPaymentNotificationWorker(notificationSecret)
	asynqMux := asynq.NewServeMux()
	worker.RegisterWorker(asynqMux, paymentWorker)

	// Start Asynq worker in background
	go func() {
		srv := queue.NewServer(redisAddr, redisPassword, 0)
		if err := srv.Run(asynqMux); err != nil {
			log.Printf("Asynq worker error: %v", err)
		}
	}()

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
	snapGroup := e.Group("/openapi/v1.0")
	snapGroup.Use(customMiddleware.IdempotencyMiddleware(redisClient, idempotencyLockTTL, idempotencyCacheTTL))
	snapGroup.POST("/access-token/b2b", tokenHandler.GetB2BAccessToken)

	// Admin: client onboarding (register client_apps / client_keys)
	adminGroup := e.Group("/admin")
	adminGroup.Use(customMiddleware.AdminAuthMiddleware(adminAPIKey))
	adminGroup.POST("/clients", clientHandler.RegisterClient)
	adminGroup.POST("/clients/:clientId/keys", clientHandler.AddClientKey)
	adminGroup.DELETE("/clients/:clientId/keys/:keyId", clientHandler.RevokeClientKey)
	if adminAPIKey == "" {
		log.Println("Warning: ADMIN_API_KEY not set — /admin/* endpoints are disabled")
	}

	// SNAP Security utility endpoints (signature helpers, no idempotency required)
	utilGroup := e.Group("/api/v1/utilities")
	utilGroup.POST("/signature-auth", signatureHandler.GenerateAccessTokenSignature)
	utilGroup.POST("/signature-service", signatureHandler.GenerateServiceSignature)

	// Register vendor-specific routes (unified under /openapi/v1.0/transfer-va/*)
	transferVAGroup := e.Group("/openapi/v1.0/transfer-va")
	transferVAGroup.Use(customMiddleware.IdempotencyMiddleware(redisClient, idempotencyLockTTL, idempotencyCacheTTL))

	// Existing SNAP VA endpoints (inquiry, payment, status)
	for _, vc := range vendorConfigs {
		vendorGroup := transferVAGroup.Group("")
		vendorGroup.Use(customMiddleware.SNAPAuthMiddleware(vc))
		vendorGroup.POST("/inquiry", vaHandler.Inquiry)
		vendorGroup.POST("/payment", vaHandler.Payment)
		vendorGroup.POST("/status", vaHandler.Status)
		log.Printf("Registered vendor routes for: %s/%s", vc.Vendor, vc.Channel)
	}

	// Default routes if no vendor configs
	if len(vendorConfigs) == 0 {
		log.Println("No vendor configs found, using default vendor VA routes")
		transferVAGroup.POST("/inquiry", vaHandler.Inquiry)
		transferVAGroup.POST("/payment", vaHandler.Payment)
		transferVAGroup.POST("/status", vaHandler.Status)
	}

	// Merchant VA Dashboard endpoints (SNAP ASPI compliant)
	transferVAGroup.POST("/create-va", merchantVAHandler.CreateVA)
	transferVAGroup.POST("/list", merchantVAHandler.ListVA)
	transferVAGroup.DELETE("/delete-va", merchantVAHandler.DeleteVA)

	log.Println("Registered merchant VA routes: create-va, list, delete-va")

	port := getEnvOrDefault("PORT", "8080")
	log.Printf("Starting SNAP Payment Gateway Server on port %s...", port)
	if err := e.Start(fmt.Sprintf(":%s", port)); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
}
