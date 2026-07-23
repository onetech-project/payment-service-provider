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

// UpdateVAStatus updates the status of a VA transaction by virtual account number
func (r *VARepository) UpdateVAStatus(ctx context.Context, virtualAccountNo string, status string) error {
	query := `
		UPDATE va_transactions 
		SET status = $2, updated_at = NOW()
		WHERE virtual_account_no = $1`

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

// GetVAByVirtualAccountNo retrieves a VA inquiry record by virtual account number
func (r *VARepository) GetVAByVirtualAccountNo(ctx context.Context, virtualAccountNo string) (*domain.VAInquiryRecord, error) {
	query := `
		SELECT id, partner_service_id, customer_no, virtual_account_no, 
			inquiry_request_id, status, total_amount, currency, created_at, updated_at
		FROM va_transactions
		WHERE virtual_account_no = $1`

	record := &domain.VAInquiryRecord{}
	err := r.pool.QueryRow(ctx, query, virtualAccountNo).Scan(
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
		return nil, domain.ErrMerchantVANotFound
	}
	if err != nil {
		return nil, err
	}
	return record, nil
}
