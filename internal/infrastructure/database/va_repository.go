package database

import (
	"context"
	"time"

	"backbone-new/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/google/uuid"
)

// VARepository implements domain.VARepository using PostgreSQL
type VARepository struct {
	pool *pgxpool.Pool
}

// NewVARepository creates a new VA repository
func NewVARepository(pool *pgxpool.Pool) *VARepository {
	return &VARepository{pool: pool}
}

// SaveInquiry saves a VA inquiry record
func (r *VARepository) SaveInquiry(ctx context.Context, inquiry *domain.VAInquiryRecord) error {
	if inquiry.ID == "" {
		inquiry.ID = uuid.New().String()
	}
	if inquiry.CreatedAt.IsZero() {
		inquiry.CreatedAt = time.Now()
	}
	inquiry.UpdatedAt = time.Now()

	query := `
		INSERT INTO va_transactions (id, partner_service_id, customer_no, virtual_account_no, 
			inquiry_request_id, status, total_amount, currency, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (inquiry_request_id) DO UPDATE SET
			status = EXCLUDED.status,
			updated_at = EXCLUDED.updated_at`

	_, err := r.pool.Exec(ctx, query,
		inquiry.ID,
		inquiry.PartnerServiceID,
		inquiry.CustomerNo,
		inquiry.VirtualAccountNo,
		inquiry.InquiryRequestID,
		inquiry.Status,
		inquiry.TotalAmount,
		inquiry.Currency,
		inquiry.CreatedAt,
		inquiry.UpdatedAt,
	)
	return err
}

// GetInquiry retrieves a VA inquiry by inquiry request ID
func (r *VARepository) GetInquiry(ctx context.Context, inquiryRequestID string) (*domain.VAInquiryRecord, error) {
	query := `
		SELECT id, partner_service_id, customer_no, virtual_account_no, 
			inquiry_request_id, status, total_amount, currency, created_at, updated_at
		FROM va_transactions
		WHERE inquiry_request_id = $1`

	record := &domain.VAInquiryRecord{}
	err := r.pool.QueryRow(ctx, query, inquiryRequestID).Scan(
		&record.ID,
		&record.PartnerServiceID,
		&record.CustomerNo,
		&record.VirtualAccountNo,
		&record.InquiryRequestID,
		&record.Status,
		&record.TotalAmount,
		&record.Currency,
		&record.CreatedAt,
		&record.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrVAInvalidBill
	}
	if err != nil {
		return nil, err
	}
	return record, nil
}

// SavePayment saves a VA payment record
func (r *VARepository) SavePayment(ctx context.Context, payment *domain.VAPaymentRecord) error {
	if payment.ID == "" {
		payment.ID = uuid.New().String()
	}
	if payment.CreatedAt.IsZero() {
		payment.CreatedAt = time.Now()
	}
	payment.UpdatedAt = time.Now()

	query := `
		INSERT INTO va_transactions (id, partner_service_id, customer_no, virtual_account_no, 
			inquiry_request_id, payment_request_id, status, total_amount, paid_amount, 
			currency, reference_no, transaction_date, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		ON CONFLICT (inquiry_request_id) DO UPDATE SET
			payment_request_id = EXCLUDED.payment_request_id,
			status = EXCLUDED.status,
			paid_amount = EXCLUDED.paid_amount,
			reference_no = EXCLUDED.reference_no,
			transaction_date = EXCLUDED.transaction_date,
			updated_at = EXCLUDED.updated_at`

	_, err := r.pool.Exec(ctx, query,
		payment.ID,
		payment.PartnerServiceID,
		payment.CustomerNo,
		payment.VirtualAccountNo,
		payment.InquiryRequestID,
		payment.PaymentRequestID,
		payment.Status,
		payment.PaidAmount, // Using paid_amount as total_amount for payment
		payment.PaidAmount,
		payment.Currency,
		payment.ReferenceNo,
		payment.TransactionDate,
		payment.CreatedAt,
		payment.UpdatedAt,
	)
	return err
}

// GetPayment retrieves a VA payment by payment request ID
func (r *VARepository) GetPayment(ctx context.Context, paymentRequestID string) (*domain.VAPaymentRecord, error) {
	query := `
		SELECT id, partner_service_id, customer_no, virtual_account_no, 
			inquiry_request_id, payment_request_id, paid_amount, currency, 
			status, reference_no, transaction_date, created_at, updated_at
		FROM va_transactions
		WHERE payment_request_id = $1 OR inquiry_request_id = $1`

	record := &domain.VAPaymentRecord{}
	err := r.pool.QueryRow(ctx, query, paymentRequestID).Scan(
		&record.ID,
		&record.PartnerServiceID,
		&record.CustomerNo,
		&record.VirtualAccountNo,
		&record.InquiryRequestID,
		&record.PaymentRequestID,
		&record.PaidAmount,
		&record.Currency,
		&record.Status,
		&record.ReferenceNo,
		&record.TransactionDate,
		&record.CreatedAt,
		&record.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrVAInvalidBill
	}
	if err != nil {
		return nil, err
	}
	return record, nil
}

// UpdatePaymentStatus updates the status of a payment
func (r *VARepository) UpdatePaymentStatus(ctx context.Context, paymentRequestID string, status string) error {
	query := `
		UPDATE va_transactions 
		SET status = $2, updated_at = NOW()
		WHERE payment_request_id = $1 OR inquiry_request_id = $1`

	result, err := r.pool.Exec(ctx, query, paymentRequestID, status)
	if err != nil {
		return err
	}
	rows := result.RowsAffected()
	if rows == 0 {
		return domain.ErrVAInvalidBill
	}
	return nil
}
