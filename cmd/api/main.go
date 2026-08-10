package main

import (
	"context"

	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"backbone-new/internal/adapter/delivery/http/handler"
	customMiddleware "backbone-new/internal/adapter/delivery/http/middleware"
	"backbone-new/internal/adapter/delivery/worker"
	"backbone-new/internal/domain"
	"backbone-new/internal/infrastructure/cache"
	"backbone-new/internal/infrastructure/config"
	"backbone-new/internal/infrastructure/crypto"
	"backbone-new/internal/infrastructure/database"
	"backbone-new/internal/infrastructure/queue"
	"backbone-new/internal/infrastructure/redis"
	"backbone-new/internal/infrastructure/telemetry"
	"backbone-new/internal/usecase"

	// docs is the swaggo-generated OpenAPI spec package produced by
	// `make swagger` (swag init -g cmd/api/main.go --output docs). Generated
	// code, committed for convenience — do NOT hand-edit anything under
	// docs/; re-run `make swagger` after changing handler annotations.
	"backbone-new/docs"

	"github.com/hibiken/asynq"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	echoSwagger "github.com/swaggo/echo-swagger"
	"github.com/swaggo/swag"
)

// healthCheckHandler godoc
// @Tags Health
// @Summary Service health check
// @Description Returns 200 with a static status payload if the service process is up. Does not check downstream dependencies (DB/Redis).
// @Success 200 {object} map[string]string
// @Router /health [get]
func healthCheckHandler(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{
		"status":  "UP",
		"service": "payment-integration-gateway",
	})
}

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

// splitAndTrim splits a comma-separated list (e.g. CORS_ALLOWED_ORIGINS) into
// trimmed, non-empty entries.
func splitAndTrim(csv string) []string {
	var out []string
	for _, part := range strings.Split(csv, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
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

// @title SNAP Payment Integration Gateway API
// @version 1.0
// @description Generic SNAP/ASPI-compliant payment gateway API: B2B access
// @description token issuance, admin client onboarding, signature-generation
// @description utilities, and Virtual Account (transfer-VA) transaction and
// @description merchant-dashboard endpoints.
// @BasePath /
//
// @securityDefinitions.apikey SnapClientKey
// @in header
// @name X-CLIENT-KEY
// @description Client identifier issued by the admin during onboarding. Required on the B2B access-token endpoint.
//
// @securityDefinitions.apikey SnapTimestamp
// @in header
// @name X-TIMESTAMP
// @description Request timestamp in ISO 8601 format (e.g. 2026-07-24T10:00:00+07:00).
//
// @securityDefinitions.apikey SnapSignature
// @in header
// @name X-SIGNATURE
// @description Request signature. Compute via POST /api/v1/utilities/signature-auth (asymmetric, access-token endpoint) or POST /api/v1/utilities/signature-service (symmetric, transaction endpoints).
//
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Bearer access token issued by POST /openapi/v1.0/access-token/b2b, e.g. "Bearer <token>".
//
// @securityDefinitions.apikey AdminToken
// @in header
// @name X-Admin-API-Key
// @description Static admin API key configured via the ADMIN_API_KEY environment variable. Required for all /admin/* endpoints.
func main() {
	// Load .env into the process environment if present. Missing file is not
	// an error (e.g. production relying on real env vars set by the platform).
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		log.Printf(".env load warning: %v", err)
	}

	ctx := context.Background()

	// load ENV (dev,uat,prod)
	appEnv := getEnvOrDefault("APP_ENV", "dev")
	// skipTimestampSkewCheck disables the ±5 minute X-TIMESTAMP freshness
	// check across the B2B token endpoint and both SNAP/merchant auth
	// middlewares, so local dev/UAT testing with stale sample requests
	// doesn't get rejected. Never skipped in prod.
	skipTimestampSkewCheck := appEnv == "dev" || appEnv == "uat"

	// Merchant signatures now hash the MINIFIED body, matching the vendor side
	// and SNAP. Defaults to also accepting the previous raw-body digest so the
	// change cannot break a merchant at deploy time; set
	// MERCHANT_LEGACY_BODY_HASH=false once every merchant has migrated. Only
	// merchants sending whitespace-bearing JSON are affected at all — for a
	// compact body the two digests are identical.
	acceptLegacyMerchantBodyHash := getEnvOrDefault("MERCHANT_LEGACY_BODY_HASH", "true") != "false"

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
	// replayed for a repeated X-EXTERNAL-ID.
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
	tokenUsecase := usecase.NewTokenUsecase(clientRepo, rsaVerifier, jwtIssuer, skipTimestampSkewCheck)
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
		// Locker-aware constructor: guards static/dynamic VA customerNo
		// sequence generation and static registration (feature
		// 006-static-dynamic-va) with the same Redis distributed lock used by
		// the idempotency middleware.
		vaRepo = database.NewVARepositoryWithLocker(pgPool, redisClient)
	}
	var notifier domain.NotificationEnqueuer
	if asynqClient != nil {
		notifier = asynqClient
	}
	// Notification delivery-attempt audit repository (feature
	// 007-merchant-expiry-callback): reuses vaRepo's concrete type, which
	// implements domain.VANotificationDeliveryRepository.
	var deliveryRepo domain.VANotificationDeliveryRepository
	if vaRepo != nil {
		deliveryRepo = vaRepo
	}
	vaUsecase := usecase.NewVAUsecaseWithDeliveryRepo(vaRepo, notifier, deliveryRepo)
	vaHandler := handler.NewVAHandler(vaUsecase)

	// Resend Callback (admin) Usecase & Handler (feature 007-merchant-expiry-callback, US2)
	resendCallbackUsecase := usecase.NewResendCallbackUsecase(vaRepo, deliveryRepo, notifier)
	adminResendHandler := handler.NewAdminResendHandler(resendCallbackUsecase)

	// VA Type Rule Provider (feature 006-static-dynamic-va amendment):
	// master_va_type / master_partner_service_ids in PostgreSQL, cached in
	// Redis with a 5-minute scheduled refresh + immediate refresh on write.
	var vaTypeRuleProvider *cache.CachedVATypeRuleProvider
	if pgPool != nil {
		masterDataRepo := database.NewMasterVADataRepository(pgPool)
		masterDataCache := redis.NewMasterDataCache(redisClient)
		vaTypeRuleProvider = cache.NewCachedVATypeRuleProvider(masterDataRepo, masterDataCache)
		if err := vaTypeRuleProvider.RefreshNow(context.Background()); err != nil {
			log.Printf("Warning: initial VA type master data load failed (will retry on next request/tick): %v", err)
		}
		vaTypeRuleProvider.Start(context.Background(), cache.DefaultRefreshInterval)
	}

	// Merchant VA Usecase & Handler
	var vaTypeRuleProviderIface domain.VATypeRuleProvider
	if vaTypeRuleProvider != nil {
		vaTypeRuleProviderIface = vaTypeRuleProvider
	}
	merchantVAUsecase := usecase.NewMerchantVAUsecase(vaRepo, vaTypeRuleProviderIface)
	merchantVAHandler := handler.NewMerchantVAHandler(merchantVAUsecase)

	// Asynq Worker for payment notifications
	notificationSecret := getEnvOrDefault("NOTIFICATION_SECRET", "default-secret")
	paymentWorker := worker.NewPaymentNotificationWorker(notificationSecret)
	asynqMux := asynq.NewServeMux()
	worker.RegisterWorker(asynqMux, paymentWorker)

	// Load vendor configurations.
	//
	// This has to happen before the Asynq worker starts, because the
	// reconciliation sweep needs an outbound client built from one of these
	// configs and its handler must be registered on asynqMux first.
	configDir := getEnvOrDefault("CONFIG_DIR", ".")
	configLoader := config.NewVendorConfigLoader(configDir)
	vendorConfigs, err := configLoader.LoadAll()
	if err != nil {
		log.Printf("Warning: Failed to load vendor configs: %v", err)
	}

	// Vendor status reconciliation (feature 014-vendor-status-reconciliation).
	//
	// Off by default: it makes OUTBOUND calls to a vendor, so it must not
	// start running the moment someone deploys this build. Operations enables
	// it once the vendor's outbound credentials are provisioned.
	reconciler := buildReconciler(vaRepo, notifier, vendorConfigs)
	adminReconcileHandler := handler.NewAdminReconcileHandler(reconciler)
	if reconciler != nil {
		worker.RegisterReconcileWorker(asynqMux, worker.NewReconcileWorker(reconciler))
		startReconcileScheduler(redisAddr, redisPassword)
	}

	// Start Asynq worker in background
	go func() {
		srv := queue.NewServer(redisAddr, redisPassword, 0)
		if err := srv.Run(asynqMux); err != nil {
			log.Printf("Asynq worker error: %v", err)
		}
	}()

	// 6. Echo Server Setup
	e := echo.New()
	e.HideBanner = true
	e.Use(echoMiddleware.Recover())
	e.Use(customMiddleware.TelemetryMiddleware())

	// CORS: allowed origins and headers are configurable via CORS_ALLOWED_ORIGINS
	// and CORS_ALLOWED_HEADERS (comma-separated), so each environment
	// (dev/uat/prod) can whitelist its own frontend origin(s) and headers
	// without a code change.
	corsAllowedOrigins := splitAndTrim(getEnvOrDefault("CORS_ALLOWED_ORIGINS", ""))
	// This list must cover EVERY header a spec-conformant SNAP client may send,
	// not just the ones we enforce. A browser blocks the whole request when any
	// header named in the preflight's Access-Control-Request-Headers is missing
	// from our Access-Control-Allow-Headers response — even if the origin is
	// allowed and even if we would have ignored the header server-side. The
	// B2B2C entries (Authorization-Customer, X-IP-ADDRESS, X-DEVICE-ID,
	// X-LATITUDE, X-LONGITUDE) and X-ORIGIN appear in the ASPI portal's own
	// sample requests, so any client built from those samples sends them.
	defaultCORSHeaders := strings.Join([]string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization,
		"Authorization-Customer",
		"X-TIMESTAMP", "X-SIGNATURE", "X-CLIENT-KEY", "X-PARTNER-ID", "X-EXTERNAL-ID", "CHANNEL-ID",
		"X-ORIGIN", "X-IP-ADDRESS", "X-DEVICE-ID", "X-LATITUDE", "X-LONGITUDE",
		"X-Admin-API-Key"}, ",")
	corsAllowedHeaders := splitAndTrim(getEnvOrDefault("CORS_ALLOWED_HEADERS", defaultCORSHeaders))
	if len(corsAllowedOrigins) > 0 {
		e.Use(echoMiddleware.CORSWithConfig(echoMiddleware.CORSConfig{
			AllowOrigins: corsAllowedOrigins,
			AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
			AllowHeaders: corsAllowedHeaders,
		}))
	} else {
		log.Println("Warning: CORS_ALLOWED_ORIGINS not set — CORS is disabled (cross-origin browser requests will be blocked)")
	}

	// Health check endpoint
	e.GET("/health", healthCheckHandler)

	// Prometheus scrape endpoint (see internal/infrastructure/telemetry/metrics.go
	// for the collectors registered against the default registry, recorded by
	// customMiddleware.TelemetryMiddleware on every request).
	e.GET("/metrics", echo.WrapHandler(promhttp.Handler()))

	// Swagger UI, served from the docs generated by `make swagger`.
	e.GET("/swagger/*", echoSwagger.WrapHandler)

	// In prod, only the "/openapi/v1.0" prefix mandated by the SNAP/ASPI
	// spec is exposed. In dev/uat, the same routes are additionally
	// mirrored under "/api/v1.0" and "/v1.0" for easier testing.
	// statusRoute is the sub-path of the inquiry-status service, registered
	// under both the v1.0 and v2.0 base paths.
	const statusRoute = "/status"

	snapBasePaths := []string{"/openapi/v1.0"}
	if appEnv == "dev" || appEnv == "uat" {
		snapBasePaths = []string{"/openapi/v1.0", "/api/v1.0", "/v1.0"}
	}

	// Developer API BCA versions the inquiry-status service at v2.0 while
	// inquiry and payment stay at v1.0, so the endpoint we expose is mirrored
	// there to match the shape vendors expect. v1.0 is kept registered
	// alongside it so vendors already pointed there are not broken.
	//
	// Note this is for ASPI-generic vendors that put service 26 on the PJP.
	// BCA itself never calls it — service 26 runs partner→BCA, and our side of
	// that is the outbound reconciler, not this route. See
	// internal/domain/reconciliation.go for why the two directions differ.
	statusBasePaths := []string{"/openapi/v2.0"}
	if appEnv == "dev" || appEnv == "uat" {
		statusBasePaths = []string{"/openapi/v2.0", "/api/v2.0", "/v2.0"}
	}

	// Mirror the extra base paths into the generated Swagger spec too, so
	// the Swagger UI lists all active routes instead of just /openapi/v1.0.
	if len(snapBasePaths) > 1 {
		if expanded, err := mirrorSwaggerPaths(snapBasePaths[0], snapBasePaths[1:]); err != nil {
			log.Printf("Warning: failed to mirror swagger paths: %v", err)
		} else {
			docs.SwaggerInfo.SwaggerTemplate = expanded
		}
	}

	// Admin: client onboarding (register client_apps / client_keys)
	adminGroup := e.Group("/admin")
	adminGroup.Use(customMiddleware.AdminAuthMiddleware(adminAPIKey))
	adminGroup.POST("/clients", clientHandler.RegisterClient)
	adminGroup.POST("/clients/:clientId/keys", clientHandler.AddClientKey)
	adminGroup.DELETE("/clients/:clientId/keys/:keyId", clientHandler.RevokeClientKey)
	adminGroup.POST("/clients/:clientId/secret", clientHandler.AddClientSecret)
	adminGroup.DELETE("/clients/:clientId/secret/:secretId", clientHandler.RevokeClientSecret)
	adminGroup.POST("/transactions/:virtualAccountNo/resend-callback", adminResendHandler.Resend)
	// On-demand counterpart of the periodic reconciliation sweep: asks the
	// vendor what really happened to a transaction still recorded as pending.
	adminGroup.POST("/transactions/:virtualAccountNo/reconcile", adminReconcileHandler.Reconcile)
	if adminAPIKey == "" {
		log.Println("Warning: ADMIN_API_KEY not set — /admin/* endpoints are disabled")
	}

	// SNAP Security utility endpoints (signature helpers, no idempotency required)
	utilGroup := e.Group("/api/v1/utilities")
	utilGroup.POST("/signature-auth", signatureHandler.GenerateAccessTokenSignature)
	utilGroup.POST("/signature-service", signatureHandler.GenerateServiceSignature)

	for _, snapBasePath := range snapBasePaths {
		// SNAP Token Endpoint
		snapGroup := e.Group(snapBasePath)
		snapGroup.POST("/access-token/b2b", tokenHandler.GetB2BAccessToken)

		// Register vendor-specific routes (unified under {snapBasePath}/transfer-va/*)
		transferVAGroup := e.Group(snapBasePath + "/transfer-va")
		// Payment is exempt from cached-response replay: a resubmit of the same
		// X-EXTERNAL-ID + paymentRequestId must reach the handler so it can
		// answer 4042518 Inconsistent Request against the stored payment,
		// rather than replaying the original 2002500 as a second success.
		transferVAGroup.Use(customMiddleware.IdempotencyMiddleware(
			redisClient, idempotencyLockTTL, idempotencyCacheTTL,
			customMiddleware.WithReplaySuppressedFor(func(c echo.Context) bool {
				return strings.HasSuffix(c.Path(), "/transfer-va/payment")
			}),
		))

		// SNAP VA endpoints (inquiry, payment, status), registered ONCE for
		// all vendors. Registering them per vendor does not work: echo keeps
		// only the last route registered for a method+path, so every vendor
		// but the last became unreachable. MultiVendorSNAPAuth resolves the
		// vendor from the request instead, and records it on the context for
		// the handler to apply that vendor's own field rules.
		if len(vendorConfigs) > 0 {
			vendorGroup := transferVAGroup.Group("")
			vendorGroup.Use(customMiddleware.MultiVendorSNAPAuth(vendorConfigs, jwtIssuer, skipTimestampSkewCheck))
			vendorGroup.POST("/inquiry", vaHandler.Inquiry)
			vendorGroup.POST("/payment", vaHandler.Payment)
			vendorGroup.POST(statusRoute, vaHandler.Status)
			for _, vc := range vendorConfigs {
				log.Printf("Registered vendor: %s/%s under %s", vc.Vendor, vc.Channel, snapBasePath)
			}
		} else {
			log.Println("No vendor configs found, using default vendor VA routes")
			transferVAGroup.POST("/inquiry", vaHandler.Inquiry)
			transferVAGroup.POST("/payment", vaHandler.Payment)
			transferVAGroup.POST(statusRoute, vaHandler.Status)
		}

		// Merchant VA Dashboard endpoints (SNAP ASPI compliant) — require a
		// valid B2B accessToken (feature 009-transfer-va-auth), isolated in
		// its own sub-group so MerchantAuthMiddleware never applies to the
		// vendor routes above (and SNAPAuthMiddleware never applies here).
		merchantGroup := transferVAGroup.Group("")
		merchantGroup.Use(customMiddleware.MerchantAuthMiddleware(jwtIssuer, clientRepo, skipTimestampSkewCheck, acceptLegacyMerchantBodyHash))
		merchantGroup.POST("/create-va", merchantVAHandler.CreateVA)
		merchantGroup.POST("/list", merchantVAHandler.ListVA)
		merchantGroup.POST("/list-transactions", merchantVAHandler.ListTransactions)
		merchantGroup.DELETE("/delete-va", merchantVAHandler.DeleteVA)

		log.Printf("Registered SNAP routes under base path: %s", snapBasePath)
	}

	// Inquiry-status at v2.0, where BCA actually calls it. Same middleware
	// chain as the v1.0 transfer-va group so idempotency and SNAP auth apply
	// identically.
	for _, statusBasePath := range statusBasePaths {
		statusGroup := e.Group(statusBasePath + "/transfer-va")
		statusGroup.Use(customMiddleware.IdempotencyMiddleware(redisClient, idempotencyLockTTL, idempotencyCacheTTL))

		if len(vendorConfigs) > 0 {
			vendorStatusGroup := statusGroup.Group("")
			vendorStatusGroup.Use(customMiddleware.MultiVendorSNAPAuth(vendorConfigs, jwtIssuer, skipTimestampSkewCheck))
			vendorStatusGroup.POST(statusRoute, vaHandler.Status)
		} else {
			statusGroup.POST(statusRoute, vaHandler.Status)
		}
		log.Printf("Registered SNAP status route under base path: %s", statusBasePath)
	}

	port := getEnvOrDefault("PORT", "8080")
	log.Printf("Starting SNAP Payment Gateway Server on port %s...", port)
	if err := e.Start(fmt.Sprintf(":%s", port)); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
}

// mirrorSwaggerPaths duplicates every generated Swagger path rooted at
// fromPrefix under each of toPrefixes, and returns the resulting spec as a
// new Swagger template. This keeps the Swagger UI in sync with the extra
// SNAP base paths that are only registered at runtime for dev/uat.
func mirrorSwaggerPaths(fromPrefix string, toPrefixes []string) (string, error) {
	raw, err := swag.ReadDoc(docs.SwaggerInfo.InstanceName())
	if err != nil {
		return "", fmt.Errorf("read swagger doc: %w", err)
	}

	var spec map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &spec); err != nil {
		return "", fmt.Errorf("unmarshal swagger doc: %w", err)
	}

	paths, ok := spec["paths"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("swagger doc has no paths object")
	}

	for path, item := range paths {
		if !strings.HasPrefix(path, fromPrefix) {
			continue
		}
		suffix := strings.TrimPrefix(path, fromPrefix)
		for _, toPrefix := range toPrefixes {
			paths[toPrefix+suffix] = item
		}
	}

	expanded, err := json.Marshal(spec)
	if err != nil {
		return "", fmt.Errorf("marshal expanded swagger doc: %w", err)
	}
	return string(expanded), nil
}
