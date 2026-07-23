package database

import (
	"context"
	"fmt"
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
		INSERT INTO va_transactions (id, partner_service_id, customer_no, customer_name, virtual_account_no,
			inquiry_request_id, trx_id, notification_url, status, total_amount, currency, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (inquiry_request_id) DO UPDATE SET
			status = EXCLUDED.status,
			notification_url = EXCLUDED.notification_url,
			updated_at = EXCLUDED.updated_at
		RETURNING id`

	// RETURNING id and scanning it back is required: on the ON CONFLICT path the
	// row keeps its ORIGINAL id (not the freshly generated one passed in above),
	// so callers that need the true persisted row id (e.g. to link bill details
	// via SaveBillDetails) must read it back rather than trust inquiry.ID as-is.
	return r.pool.QueryRow(ctx, query,
		inquiry.ID,
		inquiry.PartnerServiceID,
		inquiry.CustomerNo,
		inquiry.CustomerName,
		inquiry.VirtualAccountNo,
		inquiry.InquiryRequestID,
		inquiry.TrxID,
		inquiry.NotificationURL,
		inquiry.Status,
		inquiry.TotalAmount,
		inquiry.Currency,
		inquiry.CreatedAt,
		inquiry.UpdatedAt,
	).Scan(&inquiry.ID)
}

// GetInquiry retrieves a VA inquiry by inquiry request ID
func (r *VARepository) GetInquiry(ctx context.Context, inquiryRequestID string) (*domain.VAInquiryRecord, error) {
	query := `
		SELECT id, partner_service_id, customer_no, customer_name, virtual_account_no,
			inquiry_request_id, trx_id, notification_url, status, total_amount, currency, created_at, updated_at
		FROM va_transactions
		WHERE inquiry_request_id = $1`

	record := &domain.VAInquiryRecord{}
	err := r.pool.QueryRow(ctx, query, inquiryRequestID).Scan(
		&record.ID,
		&record.PartnerServiceID,
		&record.CustomerNo,
		&record.CustomerName,
		&record.VirtualAccountNo,
		&record.InquiryRequestID,
		&record.TrxID,
		&record.NotificationURL,
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
		INSERT INTO va_transactions (id, partner_service_id, customer_no, customer_name, virtual_account_no,
			inquiry_request_id, trx_id, notification_url, payment_request_id, status, total_amount, paid_amount,
			currency, reference_no, transaction_date, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
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
		payment.CustomerName,
		payment.VirtualAccountNo,
		payment.InquiryRequestID,
		payment.TrxID,
		payment.NotificationURL,
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

// ListVA returns a paginated list of VA transactions
func (r *VARepository) ListVA(ctx context.Context, filter *domain.VAListFilter) ([]domain.VAListItem, int, error) {
	where := "WHERE 1=1"
	args := []interface{}{}
	argIdx := 1

	if filter.PartnerServiceID != "" {
		where += " AND partner_service_id = $" + fmt.Sprintf("%d", argIdx)
		args = append(args, filter.PartnerServiceID)
		argIdx++
	}
	if filter.FromDate != nil {
		where += " AND created_at >= $" + fmt.Sprintf("%d", argIdx)
		args = append(args, *filter.FromDate)
		argIdx++
	}
	if filter.ToDate != nil {
		where += " AND created_at <= $" + fmt.Sprintf("%d", argIdx)
		args = append(args, *filter.ToDate)
		argIdx++
	}
	if filter.Status != "" {
		where += " AND status = $" + fmt.Sprintf("%d", argIdx)
		args = append(args, filter.Status)
		argIdx++
	}
	if filter.VirtualAccountNo != "" {
		where += " AND virtual_account_no = $" + fmt.Sprintf("%d", argIdx)
		args = append(args, filter.VirtualAccountNo)
		argIdx++
	}

	countQuery := "SELECT COUNT(*) FROM va_transactions " + where
	var total int
	err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	dataQuery := `
		SELECT virtual_account_no, customer_no, customer_name, total_amount, paid_amount,
			status, expired_date, created_at, transaction_date
		FROM va_transactions ` + where + `
		ORDER BY created_at DESC
		LIMIT $` + fmt.Sprintf("%d", argIdx) + ` OFFSET $` + fmt.Sprintf("%d", argIdx+1)

	args = append(args, filter.Limit, filter.Offset)
	argIdx += 2

	rows, err := r.pool.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []domain.VAListItem
	for rows.Next() {
		var item domain.VAListItem
		var totalAmount, paidAmount *string
		var expiredDate, transactionDate *time.Time
		err := rows.Scan(
			&item.VirtualAccountNo,
			&item.CustomerNo,
			&item.CustomerName,
			&totalAmount,
			&paidAmount,
			&item.Status,
			&expiredDate,
			&item.CreatedAt,
			&transactionDate,
		)
		if err != nil {
			return nil, 0, err
		}
		if totalAmount != nil {
			item.TotalAmount = &domain.Amount{Value: *totalAmount, Currency: "IDR"}
		}
		if paidAmount != nil {
			item.PaidAmount = &domain.Amount{Value: *paidAmount, Currency: "IDR"}
		}
		item.ExpiredDate = expiredDate
		item.TransactionDate = transactionDate
		items = append(items, item)
	}

	return items, total, nil
}

// GetVABillDetails returns bill details for a VA transaction
func (r *VARepository) GetVABillDetails(ctx context.Context, transactionID string) ([]domain.BillDetail, error) {
	query := `
		SELECT bill_code, bill_no, bill_name, bill_short_name,
			bill_description_en, bill_description_id, bill_sub_company,
			bill_amount, bill_amount_currency, bill_amount_label, bill_amount_value,
			bill_reference_no, biller_reference_id, status, reason_en, reason_id
		FROM va_bill_details
		WHERE transaction_id = $1`

	rows, err := r.pool.Query(ctx, query, transactionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bills []domain.BillDetail
	for rows.Next() {
		var bill domain.BillDetail
		var descEn, descID, reasonEn, reasonID *string
		var amount *float64
		var amountCurrency *string
		err := rows.Scan(
			&bill.BillCode,
			&bill.BillNo,
			&bill.BillName,
			&bill.BillShortName,
			&descEn,
			&descID,
			&bill.BillSubCompany,
			&amount,
			&amountCurrency,
			&bill.BillAmountLabel,
			&bill.BillAmountValue,
			&bill.BillReferenceNo,
			&bill.BillerReferenceID,
			&bill.Status,
			&reasonEn,
			&reasonID,
		)
		if err != nil {
			return nil, err
		}
		if descEn != nil || descID != nil {
			bill.BillDescription = &domain.BilingualText{}
			if descEn != nil {
				bill.BillDescription.English = *descEn
			}
			if descID != nil {
				bill.BillDescription.Indonesia = *descID
			}
		}
		if amount != nil {
			currency := "IDR"
			if amountCurrency != nil {
				currency = *amountCurrency
			}
			bill.BillAmount = &domain.Amount{
				Value:    fmt.Sprintf("%.2f", *amount),
				Currency: currency,
			}
		}
		if reasonEn != nil || reasonID != nil {
			bill.Reason = &domain.BilingualText{}
			if reasonEn != nil {
				bill.Reason.English = *reasonEn
			}
			if reasonID != nil {
				bill.Reason.Indonesia = *reasonID
			}
		}
		bills = append(bills, bill)
	}

	return bills, nil
}

// SaveBillDetails replaces the bill details persisted for a VA transaction.
// It runs delete+insert inside a single DB transaction so a partial write
// never leaves stale and fresh bill rows mixed together (e.g. when create-va
// is retried for a still-pending VA).
func (r *VARepository) SaveBillDetails(ctx context.Context, transactionID string, bills []domain.BillDetail) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, "DELETE FROM va_bill_details WHERE transaction_id = $1", transactionID); err != nil {
		return err
	}

	for _, bill := range bills {
		var descEn, descID, reasonEn, reasonID *string
		if bill.BillDescription != nil {
			if bill.BillDescription.English != "" {
				descEn = &bill.BillDescription.English
			}
			if bill.BillDescription.Indonesia != "" {
				descID = &bill.BillDescription.Indonesia
			}
		}
		if bill.Reason != nil {
			if bill.Reason.English != "" {
				reasonEn = &bill.Reason.English
			}
			if bill.Reason.Indonesia != "" {
				reasonID = &bill.Reason.Indonesia
			}
		}
		var amount *string
		var amountCurrency *string
		if bill.BillAmount != nil {
			amount = &bill.BillAmount.Value
			amountCurrency = &bill.BillAmount.Currency
		}

		_, err := tx.Exec(ctx, `
			INSERT INTO va_bill_details (id, transaction_id, bill_code, bill_no, bill_name, bill_short_name,
				bill_description_en, bill_description_id, bill_sub_company,
				bill_amount, bill_amount_currency, bill_amount_label, bill_amount_value,
				bill_reference_no, biller_reference_id, status, reason_en, reason_id, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)`,
			uuid.New().String(),
			transactionID,
			bill.BillCode,
			bill.BillNo,
			bill.BillName,
			bill.BillShortName,
			descEn,
			descID,
			bill.BillSubCompany,
			amount,
			amountCurrency,
			bill.BillAmountLabel,
			bill.BillAmountValue,
			bill.BillReferenceNo,
			bill.BillerReferenceID,
			bill.Status,
			reasonEn,
			reasonID,
			time.Now(),
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// UpdateVAStatus updates the status of the currently PENDING transaction for
// a virtual account number (used by DeleteVA to cancel a not-yet-paid VA).
// Scoped to status = '03' because a virtualAccountNo is reusable across
// transaction cycles (see MerchantVAUsecase.CreateVA) — without this scope,
// an unconditional "WHERE virtual_account_no = $1" would also flip the
// status of older, already-completed transactions sharing the same number.
func (r *VARepository) UpdateVAStatus(ctx context.Context, virtualAccountNo string, status string) error {
	query := `
		UPDATE va_transactions
		SET status = $2, updated_at = NOW()
		WHERE virtual_account_no = $1 AND status = '03'`

	result, err := r.pool.Exec(ctx, query, virtualAccountNo, status)
	if err != nil {
		return err
	}
	rows := result.RowsAffected()
	if rows == 0 {
		return domain.ErrMerchantVANotFound
	}
	return nil
}

// GetVAByVirtualAccountNo retrieves the MOST RECENT VA transaction for a
// virtual account number. A virtualAccountNo is reusable across transaction
// cycles (see MerchantVAUsecase.CreateVA), so multiple rows can share the
// same virtual_account_no — without ORDER BY, Postgres does not guarantee
// which one comes back, which would let a stale/completed row shadow the
// current transaction.
func (r *VARepository) GetVAByVirtualAccountNo(ctx context.Context, virtualAccountNo string) (*domain.VAInquiryRecord, error) {
	query := `
		SELECT id, partner_service_id, customer_no, customer_name, virtual_account_no,
			inquiry_request_id, trx_id, notification_url, status, total_amount, currency, created_at, updated_at
		FROM va_transactions
		WHERE virtual_account_no = $1
		ORDER BY created_at DESC
		LIMIT 1`

	record := &domain.VAInquiryRecord{}
	err := r.pool.QueryRow(ctx, query, virtualAccountNo).Scan(
		&record.ID,
		&record.PartnerServiceID,
		&record.CustomerNo,
		&record.CustomerName,
		&record.VirtualAccountNo,
		&record.InquiryRequestID,
		&record.TrxID,
		&record.NotificationURL,
		&record.Status,
		&record.TotalAmount,
		&record.Currency,
		&record.CreatedAt,
		&record.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrMerchantVANotFound
	}
	if err != nil {
		return nil, err
	}
	return record, nil
}
