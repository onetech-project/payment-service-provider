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
