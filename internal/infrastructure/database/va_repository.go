package database

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"backbone-new/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// vaLocker is the minimal distributed-lock surface this repository needs from
// internal/infrastructure/redis.Client, kept as a local interface so this
// package doesn't force a hard dependency on the redis client type in tests.
type vaLocker interface {
	AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, error)
	ReleaseLock(ctx context.Context, key string) error
}

// VARepository implements domain.VARepository using PostgreSQL
type VARepository struct {
	pool     *pgxpool.Pool
	locker   vaLocker
	lockWait time.Duration
}

// NewVARepository creates a new VA repository
func NewVARepository(pool *pgxpool.Pool) *VARepository {
	return &VARepository{pool: pool}
}

// NewVARepositoryWithLocker creates a new VA repository with a distributed
// locker for the static/dynamic VA customerNo generation/registration
// (feature 006-static-dynamic-va). Falls back to no locking (single-node
// safety only, via the DB's SELECT ... FOR UPDATE) if locker is nil.
func NewVARepositoryWithLocker(pool *pgxpool.Pool, locker vaLocker) *VARepository {
	return &VARepository{pool: pool, locker: locker}
}

const (
	sequenceLockTTL         = 5 * time.Second
	sequenceLockRetry       = 50 * time.Millisecond
	defaultSequenceLockWait = 3 * time.Second
)

// lockWaitOverride sets a non-default lock-acquisition timeout (test-only
// hook, e.g. to keep tests fast without waiting on the production default).
func (r *VARepository) lockWaitOverride(d time.Duration) {
	r.lockWait = d
}

// withLock runs fn while holding a distributed lock on key, retrying
// acquisition for up to the configured lock-wait timeout before giving up. If
// no locker is configured, fn runs unlocked (relying solely on DB-level row
// locking).
func (r *VARepository) withLock(ctx context.Context, key string, fn func() error) error {
	if r.locker == nil {
		return fn()
	}

	wait := r.lockWait
	if wait == 0 {
		wait = defaultSequenceLockWait
	}
	deadline := time.Now().Add(wait)
	for {
		ok, err := r.locker.AcquireLock(ctx, key, sequenceLockTTL)
		if err != nil {
			return fmt.Errorf("lock unavailable: %w", err)
		}
		if ok {
			defer func() { _ = r.locker.ReleaseLock(ctx, key) }()
			return fn()
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("lock timeout for key %s", key)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(sequenceLockRetry):
		}
	}
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

	// sub_company is only overwritten when the incoming record actually carries
	// one: a payment notification (SavePayment) may already have stored the
	// vendor's subCompany on this row, and a later create-va/inquiry upsert that
	// simply has nothing to say about it must not blank that value out.
	query := `
		INSERT INTO va_transactions (id, partner_service_id, customer_no, customer_name, virtual_account_no,
			inquiry_request_id, trx_id, notification_url, status, total_amount, currency, va_type, sub_company,
			expired_date, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		ON CONFLICT (inquiry_request_id) DO UPDATE SET
			status = EXCLUDED.status,
			notification_url = EXCLUDED.notification_url,
			sub_company = COALESCE(NULLIF(EXCLUDED.sub_company, ''), va_transactions.sub_company),
			expired_date = EXCLUDED.expired_date,
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
		inquiry.VAType,
		inquiry.SubCompany,
		inquiry.ExpiredDate,
		inquiry.CreatedAt,
		inquiry.UpdatedAt,
	).Scan(&inquiry.ID)
}

// GetInquiry retrieves a VA inquiry by inquiry request ID
func (r *VARepository) GetInquiry(ctx context.Context, inquiryRequestID string) (*domain.VAInquiryRecord, error) {
	// Selects the same column set as GetVAByVirtualAccountNo: both feed the same
	// VAUsecase.Inquiry response builder, which needs sub_company, va_type and
	// expired_date to derive subCompany and the expiry outcome — a record
	// reached by inquiryRequestId must not answer differently from the very same
	// row reached by virtualAccountNo.
	query := `
		SELECT id, partner_service_id, customer_no, customer_name, virtual_account_no,
			inquiry_request_id, trx_id, notification_url, status, total_amount, currency,
			COALESCE(va_type, ''), COALESCE(sub_company, ''), expired_date, created_at, updated_at
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
		&record.VAType,
		&record.SubCompany,
		&record.ExpiredDate,
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

	channelCode := ""
	if payment.ChannelCode != 0 {
		channelCode = strconv.Itoa(payment.ChannelCode)
	}

	// total_amount preserves the original bill total for a brand-new orphan
	// row with no prior inquiry (falls back to paid_amount); on the common
	// ON CONFLICT path total_amount is intentionally left out of DO UPDATE SET
	// so the inquiry's original total is never overwritten by a payment.
	totalAmount := payment.TotalAmount
	if totalAmount == "" {
		totalAmount = payment.PaidAmount
	}

	var freeTexts []byte
	if len(payment.FreeTexts) > 0 {
		var err error
		freeTexts, err = json.Marshal(payment.FreeTexts)
		if err != nil {
			return err
		}
	}

	query := `
		INSERT INTO va_transactions (id, partner_service_id, customer_no, customer_name, customer_email,
			customer_phone, virtual_account_no, inquiry_request_id, trx_id, notification_url, payment_request_id,
			status, total_amount, paid_amount, currency, reference_no, channel_code, hashed_source_account_no,
			source_bank_code, journal_num, payment_type, flag_advise, paid_bills, sub_company, trx_date_time,
			free_texts, transaction_date, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21,
			$22, $23, $24, $25, $26, $27, $28, $29)
		ON CONFLICT (inquiry_request_id) DO UPDATE SET
			payment_request_id = EXCLUDED.payment_request_id,
			customer_name = COALESCE(NULLIF(va_transactions.customer_name, ''), NULLIF(EXCLUDED.customer_name, ''),
				va_transactions.customer_name),
			customer_email = EXCLUDED.customer_email,
			customer_phone = EXCLUDED.customer_phone,
			status = EXCLUDED.status,
			paid_amount = EXCLUDED.paid_amount,
			reference_no = EXCLUDED.reference_no,
			channel_code = EXCLUDED.channel_code,
			hashed_source_account_no = EXCLUDED.hashed_source_account_no,
			source_bank_code = EXCLUDED.source_bank_code,
			journal_num = EXCLUDED.journal_num,
			payment_type = EXCLUDED.payment_type,
			flag_advise = EXCLUDED.flag_advise,
			paid_bills = EXCLUDED.paid_bills,
			sub_company = EXCLUDED.sub_company,
			trx_date_time = EXCLUDED.trx_date_time,
			free_texts = EXCLUDED.free_texts,
			transaction_date = EXCLUDED.transaction_date,
			updated_at = EXCLUDED.updated_at`

	_, err := r.pool.Exec(ctx, query,
		payment.ID,
		payment.PartnerServiceID,
		payment.CustomerNo,
		payment.CustomerName,
		payment.CustomerEmail,
		payment.CustomerPhone,
		payment.VirtualAccountNo,
		payment.InquiryRequestID,
		payment.TrxID,
		payment.NotificationURL,
		payment.PaymentRequestID,
		payment.Status,
		totalAmount,
		payment.PaidAmount,
		payment.Currency,
		payment.ReferenceNo,
		channelCode,
		payment.HashedSourceAccountNo,
		payment.SourceBankCode,
		payment.JournalNum,
		payment.PaymentType,
		payment.FlagAdvise,
		payment.PaidBills,
		payment.SubCompany,
		payment.TrxDateTime,
		freeTexts,
		payment.TransactionDate,
		payment.CreatedAt,
		payment.UpdatedAt,
	)
	return err
}

// GetPayment retrieves a VA payment by payment request ID
func (r *VARepository) GetPayment(ctx context.Context, paymentRequestID string) (*domain.VAPaymentRecord, error) {
	query := `
		SELECT id, partner_service_id, customer_no, customer_name, COALESCE(customer_email, ''),
			COALESCE(customer_phone, ''), virtual_account_no, inquiry_request_id, trx_id, payment_request_id,
			COALESCE(total_amount, paid_amount), paid_amount, currency, status, reference_no,
			COALESCE(channel_code, ''), COALESCE(hashed_source_account_no, ''), COALESCE(source_bank_code, ''),
			COALESCE(journal_num, ''), COALESCE(payment_type, ''), COALESCE(flag_advise, ''),
			COALESCE(paid_bills, ''), COALESCE(sub_company, ''), trx_date_time, free_texts,
			transaction_date, created_at, updated_at
		FROM va_transactions
		WHERE payment_request_id = $1 OR inquiry_request_id = $1`

	record := &domain.VAPaymentRecord{}
	var channelCode string
	var freeTexts []byte
	err := r.pool.QueryRow(ctx, query, paymentRequestID).Scan(
		&record.ID,
		&record.PartnerServiceID,
		&record.CustomerNo,
		&record.CustomerName,
		&record.CustomerEmail,
		&record.CustomerPhone,
		&record.VirtualAccountNo,
		&record.InquiryRequestID,
		&record.TrxID,
		&record.PaymentRequestID,
		&record.TotalAmount,
		&record.PaidAmount,
		&record.Currency,
		&record.Status,
		&record.ReferenceNo,
		&channelCode,
		&record.HashedSourceAccountNo,
		&record.SourceBankCode,
		&record.JournalNum,
		&record.PaymentType,
		&record.FlagAdvise,
		&record.PaidBills,
		&record.SubCompany,
		&record.TrxDateTime,
		&freeTexts,
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
	if channelCode != "" {
		record.ChannelCode, _ = strconv.Atoi(channelCode)
	}
	if len(freeTexts) > 0 {
		_ = json.Unmarshal(freeTexts, &record.FreeTexts)
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
			inquiry_request_id, trx_id, notification_url, status, total_amount, currency,
			COALESCE(va_type, ''), COALESCE(sub_company, ''), expired_date, created_at, updated_at
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
		&record.VAType,
		&record.SubCompany,
		&record.ExpiredDate,
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

// Notification Delivery Attempt persistence (feature
// 007-merchant-expiry-callback). Exercised against a live PostgreSQL
// connection per quickstart.md's integration scenarios, consistent with this
// file's other SQL-heavy methods (NextCustomerNoSequence,
// RegisterStaticCustomerNo, SaveVAPayment).

// Create inserts a new notification delivery-attempt audit row.
func (r *VARepository) Create(ctx context.Context, delivery *domain.NotificationDelivery) error {
	if delivery.ID == "" {
		delivery.ID = uuid.New().String()
	}
	if delivery.AttemptedAt.IsZero() {
		delivery.AttemptedAt = time.Now()
	}

	var errorDetail *string
	if delivery.ErrorDetail != "" {
		errorDetail = &delivery.ErrorDetail
	}

	query := `
		INSERT INTO va_notification_deliveries (id, virtual_account_no, event_type, trigger, status, attempted_at, error_detail)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := r.pool.Exec(ctx, query,
		delivery.ID,
		delivery.VirtualAccountNo,
		delivery.EventType,
		delivery.Trigger,
		delivery.Status,
		delivery.AttemptedAt,
		errorDetail,
	)
	return err
}

// GetLatestByVirtualAccountNo returns the most recent delivery-attempt row
// (any event type/trigger) for a virtual account number, used by the resend
// endpoint to determine which event to redeliver (FR-011/FR-015).
func (r *VARepository) GetLatestByVirtualAccountNo(ctx context.Context, virtualAccountNo string) (*domain.NotificationDelivery, error) {
	query := `
		SELECT id, virtual_account_no, event_type, trigger, status, attempted_at, COALESCE(error_detail, '')
		FROM va_notification_deliveries
		WHERE virtual_account_no = $1
		ORDER BY attempted_at DESC
		LIMIT 1`

	record := &domain.NotificationDelivery{}
	err := r.pool.QueryRow(ctx, query, virtualAccountNo).Scan(
		&record.ID,
		&record.VirtualAccountNo,
		&record.EventType,
		&record.Trigger,
		&record.Status,
		&record.AttemptedAt,
		&record.ErrorDetail,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return record, nil
}

// ExistsByVirtualAccountNoAndEventType reports whether a delivery-attempt row
// already exists for the given virtual account number / event type /
// trigger combination, used to dedupe auto-triggered "va.expired" callbacks
// (FR-005).
func (r *VARepository) ExistsByVirtualAccountNoAndEventType(ctx context.Context, virtualAccountNo, eventType, trigger string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM va_notification_deliveries
			WHERE virtual_account_no = $1 AND event_type = $2 AND trigger = $3
		)`

	var exists bool
	err := r.pool.QueryRow(ctx, query, virtualAccountNo, eventType, trigger).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// NextCustomerNoSequence generates the next unique, sequential customerNo for
// a dynamic VA type (feature 006-static-dynamic-va, FR-005/FR-005a). The
// result is a 20-digit string: the 2-digit vaType followed by the 18-digit
// zero-padded sequence. Guarded by a Redis lock (if configured) plus a
// row-level lock on the counter row for defense in depth under concurrency.
func (r *VARepository) NextCustomerNoSequence(ctx context.Context, vaType string) (string, error) {
	var customerNo string
	err := r.withLock(ctx, fmt.Sprintf("va-seq-lock:%s", vaType), func() error {
		tx, err := r.pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("sequence generator unavailable: %w", err)
		}
		defer func() { _ = tx.Rollback(ctx) }()

		var nextSeq int64
		err = tx.QueryRow(ctx,
			`SELECT next_seq FROM va_customer_no_sequences WHERE va_type = $1 FOR UPDATE`,
			vaType,
		).Scan(&nextSeq)
		if err != nil {
			return fmt.Errorf("sequence generator unavailable: %w", err)
		}

		seqStr := fmt.Sprintf("%d", nextSeq)
		if len(seqStr) > 18 {
			return fmt.Errorf("sequence generator unavailable: sequence range exhausted for vaType %s", vaType)
		}

		if _, err := tx.Exec(ctx,
			`UPDATE va_customer_no_sequences SET next_seq = next_seq + 1, updated_at = NOW() WHERE va_type = $1`,
			vaType,
		); err != nil {
			return fmt.Errorf("sequence generator unavailable: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("sequence generator unavailable: %w", err)
		}

		customerNo = vaType + fmt.Sprintf("%018d", nextSeq)
		return nil
	})
	if err != nil {
		return "", err
	}
	return customerNo, nil
}

// RegisterStaticCustomerNo enforces that a merchant-supplied customerNo is
// only ever used once per partnerServiceId (feature 006-static-dynamic-va,
// FR-008). Returns domain.ErrClientAlreadyExists-style conflict on duplicate.
func (r *VARepository) RegisterStaticCustomerNo(ctx context.Context, partnerServiceID, customerNo string) error {
	return r.withLock(ctx, fmt.Sprintf("va-static-lock:%s:%s", partnerServiceID, customerNo), func() error {
		var existingCount int
		err := r.pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM va_transactions WHERE partner_service_id = $1 AND customer_no = $2`,
			partnerServiceID, customerNo,
		).Scan(&existingCount)
		if err != nil {
			return fmt.Errorf("sequence generator unavailable: %w", err)
		}
		if existingCount > 0 {
			return domain.ErrVACustomerNoAlreadyRegistered
		}
		return nil
	})
}

// SaveVAPayment records an individual payment against a variable-bill VA
// transaction, recalculates the cumulative paid_amount, and transitions the
// transaction to fully-paid ("00") once it reaches total_amount (feature
// 006-static-dynamic-va, FR-013).
func (r *VARepository) SaveVAPayment(ctx context.Context, transactionID string, amount string, referenceNo string) (paidAmount string, status string, err error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err = tx.Exec(ctx,
		`INSERT INTO va_payments (id, transaction_id, amount, reference_no) VALUES ($1, $2, $3, $4)`,
		uuid.New().String(), transactionID, amount, referenceNo,
	); err != nil {
		return "", "", err
	}

	var totalAmount string
	err = tx.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount), 0)::text FROM va_payments WHERE transaction_id = $1`,
		transactionID,
	).Scan(&paidAmount)
	if err != nil {
		return "", "", err
	}

	err = tx.QueryRow(ctx,
		`SELECT total_amount::text FROM va_transactions WHERE id = $1`,
		transactionID,
	).Scan(&totalAmount)
	if err != nil {
		return "", "", err
	}

	status = "03"
	paidVal, errPaid := parseAmount(paidAmount)
	totalVal, errTotal := parseAmount(totalAmount)
	if errPaid == nil && errTotal == nil && paidVal >= totalVal {
		status = "00"
	}

	if _, err = tx.Exec(ctx,
		`UPDATE va_transactions SET paid_amount = $2, status = $3, updated_at = NOW() WHERE id = $1`,
		transactionID, paidAmount, status,
	); err != nil {
		return "", "", err
	}

	if err = tx.Commit(ctx); err != nil {
		return "", "", err
	}

	return paidAmount, status, nil
}

// parseAmount parses a NUMERIC(16,2) string as returned by Postgres (e.g.
// "150000.00") into a float64 for cumulative-amount comparison.
func parseAmount(s string) (float64, error) {
	var v float64
	_, err := fmt.Sscanf(s, "%f", &v)
	return v, err
}

// Ensure VARepository implements domain.VARepository and
// domain.VANotificationDeliveryRepository.
var _ domain.VARepository = (*VARepository)(nil)
var _ domain.VANotificationDeliveryRepository = (*VARepository)(nil)
