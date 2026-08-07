package main

import (
	"log"
	"strconv"
	"time"

	"backbone-new/internal/adapter/delivery/worker"
	"backbone-new/internal/adapter/gateway/snap"
	"backbone-new/internal/domain"
	"backbone-new/internal/infrastructure/config"
	"backbone-new/internal/infrastructure/queue"
	"backbone-new/internal/usecase"
)

// Wiring for vendor status reconciliation
// (feature 014-vendor-status-reconciliation).
//
// Kept out of main() because every step here can legitimately decline to build
// the reconciler — disabled, no such vendor, no outbound credentials — and
// each refusal needs to say why. Threaded through main() as inline ifs it
// would have been a wall of nested error paths in an already long function.

// buildReconciler assembles the reconciler, or returns nil when this
// deployment is not configured for it.
//
// nil is a valid, expected result, not a failure: reconciliation makes
// outbound calls to a vendor, so it stays off until operations explicitly
// turns it on and provisions credentials. The admin endpoint reports the nil
// case as 503 with an explanation rather than pretending to work.
func buildReconciler(
	repo domain.ReconciliationRepository,
	notifier domain.NotificationEnqueuer,
	vendorConfigs []*config.VendorConfig,
) domain.VAStatusReconciler {
	if getEnvOrDefault("RECONCILE_ENABLED", "false") != "true" {
		return nil
	}

	vendor := getEnvOrDefault("RECONCILE_VENDOR", "bca")
	channel := getEnvOrDefault("RECONCILE_CHANNEL", "va")

	vendorConfig := findVendorConfig(vendorConfigs, vendor, channel)
	if vendorConfig == nil {
		log.Printf("Warning: RECONCILE_ENABLED=true but no vendor config found for %s/%s — reconciliation disabled", vendor, channel)
		return nil
	}

	// Without a base URL there is nowhere to send the inquiry, and without
	// outbound credentials the vendor will reject it. Both are checked here so
	// the failure is one clear startup line instead of a sweep that logs an
	// error every interval forever.
	if vendorConfig.BaseURL == "" {
		log.Printf("Warning: vendor %s/%s has no VENDOR_BASE_URL — reconciliation disabled", vendor, channel)
		return nil
	}
	if vendorConfig.ClientID == "" || vendorConfig.OutboundPrivateKeyPath == "" {
		log.Printf("Warning: vendor %s/%s has no outbound credentials (VENDOR_CLIENT_ID / VENDOR_PRIVATE_KEY_PATH) — reconciliation disabled", vendor, channel)
		return nil
	}

	gateway := snap.NewClient(vendorConfig, snap.NewTokenProvider(vendorConfig))
	cfg := usecase.ReconcileConfig{
		PendingAfter: time.Duration(getEnvIntOrDefault("RECONCILE_PENDING_AFTER_MINUTES", 15)) * time.Minute,
		BatchSize:    getEnvIntOrDefault("RECONCILE_BATCH_SIZE", 100),
		ClientID:     vendorConfig.ClientID,
	}

	log.Printf("Vendor status reconciliation enabled: vendor=%s/%s pending_after=%s batch=%d",
		vendor, channel, cfg.PendingAfter, cfg.BatchSize)

	return usecase.NewReconciliationUsecase(repo, gateway, notifier, cfg)
}

// findVendorConfig locates the config for one vendor/channel pair.
func findVendorConfig(configs []*config.VendorConfig, vendor, channel string) *config.VendorConfig {
	for _, cfg := range configs {
		if cfg.Vendor == vendor && cfg.Channel == channel {
			return cfg
		}
	}
	return nil
}

// startReconcileScheduler runs the periodic sweep enqueuer.
//
// Safe to start on every replica: asynq elects a single active scheduler
// through Redis, so one sweep is enqueued per interval regardless of how many
// instances are deployed — which is the whole reason this goes through the
// queue rather than an in-process ticker.
func startReconcileScheduler(redisAddr, redisPassword string) {
	intervalMinutes := getEnvIntOrDefault("RECONCILE_INTERVAL_MINUTES", 5)
	interval := time.Duration(intervalMinutes) * time.Minute
	spec := "@every " + strconv.Itoa(intervalMinutes) + "m"

	scheduler := queue.NewScheduler(redisAddr, redisPassword, 0)
	if err := scheduler.RegisterPeriodic(spec, worker.TaskVAStatusReconcile, interval); err != nil {
		log.Printf("Warning: failed to schedule reconciliation sweep (%s): %v", spec, err)
		return
	}

	go func() {
		if err := scheduler.Run(); err != nil {
			log.Printf("Reconciliation scheduler error: %v", err)
		}
	}()
	log.Printf("Reconciliation sweep scheduled: %s", spec)
}

// getEnvIntOrDefault reads an integer env var, falling back on absent or
// unparseable values rather than starting with a zero interval.
func getEnvIntOrDefault(key string, defaultValue int) int {
	raw := getEnvOrDefault(key, "")
	if raw == "" {
		return defaultValue
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		log.Printf("Warning: %s=%q is not a positive integer — using %d", key, raw, defaultValue)
		return defaultValue
	}
	return v
}
