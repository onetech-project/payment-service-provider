package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"backbone-new/internal/domain"

	"github.com/hibiken/asynq"
)

// Client wraps asynq.Client for task enqueuing
type Client struct {
	client *asynq.Client
}

// NewClient creates a new Asynq client connected to Redis
func NewClient(redisAddr, redisPassword string, db int) (*Client, error) {
	srv := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       db,
	})
	return &Client{client: srv}, nil
}

// EnqueuePaymentNotification enqueues a payment notification task
func (c *Client) EnqueuePaymentNotification(ctx context.Context, payload *domain.PaymentNotificationPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal notification payload: %w", err)
	}

	task := asynq.NewTask("merchant:payment:notify", data)
	info, err := c.client.EnqueueContext(ctx, task,
		asynq.Queue("notifications"),
		asynq.MaxRetry(3),
		asynq.Timeout(30*time.Second),
		asynq.Retention(24*time.Hour),
		asynq.Unique(5*time.Minute),
	)
	if err != nil {
		return fmt.Errorf("failed to enqueue notification task: %w", err)
	}

	_ = info
	return nil
}

// Close shuts down the Asynq client
func (c *Client) Close() error {
	return c.client.Close()
}

// Server wraps asynq.Server for task processing
type Server struct {
	server *asynq.Server
}

// NewServer creates a new Asynq server for processing tasks
func NewServer(redisAddr, redisPassword string, db int) *Server {
	srv := asynq.NewServer(asynq.RedisClientOpt{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       db,
	}, asynq.Config{
		Concurrency: 10,
		Queues: map[string]int{
			"notifications": 3,
			"default":       1,
		},
	})
	return &Server{server: srv}
}

// Run starts the server
func (s *Server) Run(mux *asynq.ServeMux) error {
	return s.server.Run(mux)
}

// Scheduler wraps asynq.Scheduler for periodic task enqueuing.
//
// Separate from Server on purpose: the Server processes whatever is queued,
// the Scheduler decides when something should be queued. Running the Scheduler
// on every replica is safe — asynq elects a single active scheduler through
// Redis — so exactly one sweep is enqueued per interval no matter how many
// instances are deployed.
type Scheduler struct {
	scheduler *asynq.Scheduler
}

// NewScheduler creates a periodic-task scheduler connected to Redis.
func NewScheduler(redisAddr, redisPassword string, db int) *Scheduler {
	return &Scheduler{
		scheduler: asynq.NewScheduler(asynq.RedisClientOpt{
			Addr:     redisAddr,
			Password: redisPassword,
			DB:       db,
		}, nil),
	}
}

// RegisterPeriodic schedules taskType on a cron spec (asynq also accepts the
// "@every 5m" form). interval must match the cron spec's period.
//
// interval is passed separately because it sets the uniqueness window, and
// getting that wrong is silent: a periodic task always carries the same
// payload, so asynq's unique lock is keyed identically on every firing. A
// window longer than the period therefore does not "prevent stacking", it
// throttles the schedule itself — a fixed one-hour window made an "@every 1m"
// sweep run once an hour and made the configured interval meaningless, with
// nothing logged to say so.
//
// Sized to the interval, the lock does what was intended: a firing is dropped
// only while the previous run of the same task is still in flight, so a slow
// sweep cannot stack duplicates and double the outbound traffic aimed at the
// vendor.
func (s *Scheduler) RegisterPeriodic(cronSpec, taskType string, interval time.Duration) error {
	if interval <= 0 {
		return fmt.Errorf("periodic task %s: interval must be positive, got %s", taskType, interval)
	}
	_, err := s.scheduler.Register(cronSpec, asynq.NewTask(taskType, nil),
		asynq.Unique(interval),
		asynq.MaxRetry(2),
		// Bounded well under the interval-derived unique window so a hung run
		// cannot hold the lock indefinitely and stop the schedule for good.
		asynq.Timeout(interval),
	)
	return err
}

// Run starts the scheduler (blocking).
func (s *Scheduler) Run() error {
	return s.scheduler.Run()
}

// Shutdown stops the scheduler.
func (s *Scheduler) Shutdown() {
	s.scheduler.Shutdown()
}
