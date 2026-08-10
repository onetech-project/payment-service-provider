package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"backbone-new/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// whereAlwaysTrue seeds the dynamically-built WHERE clauses in the list
// queries so every optional filter can be appended uniformly as " AND ...".
const whereAlwaysTrue = "WHERE 1=1"

// pgUniqueViolation is the SQLSTATE class Postgres raises when an INSERT
// violates a unique constraint. Used to turn a duplicate paymentRequestId
// into domain.ErrVAPaymentDuplicate rather than a generic 500.
const pgUniqueViolation = "23505"

// isUniqueViolation reports whether err is a Postgres unique-constraint
// violation.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation
}

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
	// free_texts is written as JSONB, same as SavePayment does. NULL rather
	// than an empty array when the biller set none, so COALESCE readers can
	// tell "not provided" from "provided empty".
	var freeTexts []byte
	if len(inquiry.FreeTexts) > 0 {
		var err error
		freeTexts, err = json.Marshal(inquiry.FreeTexts)
		if err != nil {
			return err
		}
	}

	query := `
		INSERT INTO va_transactions (id, partner_service_id, customer_no, customer_name, virtual_account_no,
			inquiry_request_id, trx_id, notification_url, status, total_amount, currency, va_type, sub_company,
			free_texts, expired_date, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		ON CONFLICT (inquiry_request_id) DO UPDATE SET
			status = EXCLUDED.status,
			notification_url = EXCLUDED.notification_url,
			sub_company = COALESCE(NULLIF(EXCLUDED.sub_company, ''), va_transactions.sub_company),
			-- Same reasoning as sub_company: a payment notification may have
			-- stored the vendor's freeTexts on this row, and a later upsert
			-- that carries none must not blank them out.
			free_texts = COALESCE(EXCLUDED.free_texts, va_transactions.free_texts),
			expired_date = EXCLUDED.expired_date,
			-- A VA number is reusable once its previous transaction reached a
			-- terminal state, and the new cycle may carry a different bill.
			-- Leaving these untouched kept the OLD amount and holder on the
			-- row, so the next payment was validated against the previous
			-- cycle's bill.
			customer_name = COALESCE(NULLIF(EXCLUDED.customer_name, ''), va_transactions.customer_name),
			trx_id = COALESCE(NULLIF(EXCLUDED.trx_id, ''), va_transactions.trx_id),
			va_type = COALESCE(NULLIF(EXCLUDED.va_type, ''), va_transactions.va_type),
			total_amount = EXCLUDED.total_amount,
			currency = EXCLUDED.currency,
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
		freeTexts,
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
			COALESCE(inquiry_request_id, ''), trx_id, notification_url, status, total_amount, currency,
			COALESCE(va_type, ''), COALESCE(sub_company, ''), COALESCE(paid_amount, 0)::text,
			free_texts, expired_date, created_at, updated_at
		FROM va_transactions
		WHERE inquiry_request_id = $1`

	record := &domain.VAInquiryRecord{}
	var freeTexts []byte
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
		&record.PaidAmount,
		&freeTexts,
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
	// Unmarshal errors are swallowed: freeTexts is optional display text, and a
	// row with unreadable JSON in it must still answer the inquiry rather than
	// fail it.
	if len(freeTexts) > 0 {
		_ = json.Unmarshal(freeTexts, &record.FreeTexts)
	}
	return record, nil
}

// ClaimInquiryRequestID stamps inquiryRequestID onto the row only while it is
// still unclaimed. The WHERE guard is what makes this safe to call on every
// inquiry: a row that already carries a real vendor id keeps it, so a later
// inquiry with a different id can never rewrite the value that Status and
// Payment resolve this transaction by.
//
// Unclaimed has two shapes. ” is what create-va writes. A copy of trx_id is
// what rows written before create-va stopped filling the column carry — also a
// placeholder, never a vendor-supplied id, so it is replaced the same way.
func (r *VARepository) ClaimInquiryRequestID(ctx context.Context, id string, inquiryRequestID string) error {
	// The placeholder set must stay in step with
	// domain.IsPlaceholderInquiryRequestID: '' and a copy of trx_id from
	// earlier create-va generations, and the VA number from the current one.
	// A placeholder missing from this list makes the claim a silent no-op —
	// the row keeps the placeholder, and every later Status() lookup by the
	// vendor's real inquiryRequestId reports Transaction Not Found.
	//
	// NULL is a fourth spelling, carried by rows created before create-va
	// started writing '' at all. It has to be named explicitly — NULL = '' is
	// unknown, not true, so without the IS NULL arm the UPDATE silently matches
	// 0 rows and the VA stays permanently unreachable by inquiryRequestId.
	query := `
		UPDATE va_transactions
		SET inquiry_request_id = $2, updated_at = NOW()
		WHERE id = $1
		  AND (inquiry_request_id IS NULL
		       OR inquiry_request_id = ''
		       OR inquiry_request_id = trx_id
		       OR inquiry_request_id = virtual_account_no)`

	_, err := r.pool.Exec(ctx, query, id, inquiryRequestID)
	return err
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

// paymentSelectColumns is shared by GetPayment and
// GetPaymentByPaymentRequestID so the two differ only in their WHERE clause.
//
// payment_request_id, paid_amount and reference_no are NULL on a row that has
// not been through the single-settlement payment path — a variable-bill VA
// settles through va_payments instead, and its transaction row keeps them
// NULL. Scanning those into strings failed, turning a perfectly good status
// inquiry into a 500.
const paymentSelectColumns = `
	SELECT id, partner_service_id, customer_no, customer_name, COALESCE(customer_email, ''),
		COALESCE(customer_phone, ''), virtual_account_no, inquiry_request_id, trx_id,
		COALESCE(payment_request_id, ''),
		COALESCE(total_amount, paid_amount, 0), COALESCE(paid_amount, 0), currency, status,
		COALESCE(reference_no, ''),
		COALESCE(channel_code, ''), COALESCE(hashed_source_account_no, ''), COALESCE(source_bank_code, ''),
		COALESCE(journal_num, ''), COALESCE(payment_type, ''), COALESCE(flag_advise, ''),
		COALESCE(paid_bills, ''), COALESCE(sub_company, ''), trx_date_time, free_texts,
		COALESCE(transaction_date, updated_at), created_at, updated_at
	FROM va_transactions`

// GetPayment resolves a transaction by EITHER identifier — used by the status
// service, which is handed an inquiryRequestId and must still find the payment
// that settled it.
//
// Do NOT use this as a "has this payment already been recorded?" check: see
// GetPaymentByPaymentRequestID for why the OR is actively wrong there.
func (r *VARepository) GetPayment(ctx context.Context, paymentRequestID string) (*domain.VAPaymentRecord, error) {
	return r.scanPayment(ctx,
		paymentSelectColumns+`
		WHERE payment_request_id = $1 OR inquiry_request_id = $1`,
		paymentRequestID)
}

// GetPaymentByPaymentRequestID resolves a transaction by paymentRequestId
// ONLY, and is the lookup the payment endpoint's duplicate check must use.
//
// The difference is not cosmetic. BCA specifies "paymentRequestId ... If
// payment comes from the Inquiry process, this value must be the same with
// inquiryRequestId" — so in the canonical inquiry-then-pay flow the two ids
// are equal, and the inquiry has already stamped that id onto the
// transaction's inquiry_request_id. Matching with GetPayment's OR therefore
// finds the *unpaid* transaction the inquiry just claimed and reports the
// customer's FIRST payment as a double-flag: BCA receives 4042518
// "Inconsistent Request", no payment row is written, the transaction stays
// pending and the merchant is never notified. The money is taken and never
// recorded.
func (r *VARepository) GetPaymentByPaymentRequestID(ctx context.Context, paymentRequestID string) (*domain.VAPaymentRecord, error) {
	return r.scanPayment(ctx,
		paymentSelectColumns+`
		WHERE payment_request_id = $1`,
		paymentRequestID)
}

func (r *VARepository) scanPayment(ctx context.Context, query, paymentRequestID string) (*domain.VAPaymentRecord, error) {
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

// GetVAByVirtualAccountNo retrieves the PAYABLE VA transaction for a virtual
// account number, falling back to the most recent row when none is payable.
//
// A virtualAccountNo is reusable across transaction cycles (see
// MerchantVAUsecase.CreateVA), so multiple rows can share the same
// virtual_account_no. Recency alone picks the wrong one: a VA carrying both a
// settled row and a still-open bill answers from whichever was written last,
// so an outstanding bill gets reported as 4042414 "Paid Bill" on inquiry and
// rejected 4092500 on payment purely because a completed row happens to be
// newer. Payability has to rank ahead of recency.
//
// Payable means either of:
//
//   - status '03' — pending, nothing settled it yet. For a variable-bill VA
//     this already carries "belum lunas": SaveVAPayment only flips the row to
//     '00' once the cumulative total reaches total_amount.
//
//   - status '00' on a variable-bill VA (va_type 02/05) whose payments still
//     fall short of total_amount. Such a row is marked settled but is not
//     actually lunas, and the remaining balance must stay collectable rather
//     than be locked out by a stale status. Deliberately scoped to '00': an
//     expired ('02') or deleted ('04') VA is closed for other reasons and
//     must NOT be resurrected by an amount comparison.
//
// Among payable rows the newest wins (the current bill). With none payable the
// newest row of any status is returned, so the paid/expired/deleted rejections
// keep reporting the VA the vendor actually asked about.
func (r *VARepository) GetVAByVirtualAccountNo(ctx context.Context, virtualAccountNo string) (*domain.VAInquiryRecord, error) {
	// inquiry_request_id is COALESCEd like va_type/sub_company: the column is
	// nullable and a merchant-created VA has no vendor inquiryRequestId until
	// the first inquiry claims one, so scanning it raw into VAInquiryRecord's
	// string field fails on exactly the rows this lookup exists to serve — and
	// that scan error surfaces to the vendor as a bare 5002400. Empty string is
	// the record's own "not claimed yet" marker (see VAUsecase.Inquiry).
	query := `
		SELECT id, partner_service_id, customer_no, customer_name, virtual_account_no,
			COALESCE(inquiry_request_id, ''), trx_id, notification_url, status, total_amount, currency,
			COALESCE(va_type, ''), COALESCE(sub_company, ''), COALESCE(paid_amount, 0)::text,
			free_texts, expired_date, created_at, updated_at
		FROM va_transactions
		WHERE virtual_account_no = $1
		ORDER BY
			CASE
				WHEN status = '03' THEN 0
				WHEN status = '00'
					AND COALESCE(va_type, '') IN ('02', '05')
					AND COALESCE(paid_amount, 0) < COALESCE(total_amount, 0) THEN 0
				ELSE 1
			END,
			created_at DESC
		LIMIT 1`

	record := &domain.VAInquiryRecord{}
	var freeTexts []byte
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
		&record.PaidAmount,
		&freeTexts,
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
	if len(freeTexts) > 0 {
		_ = json.Unmarshal(freeTexts, &record.FreeTexts)
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
// result is an 18-digit string: the 2-digit vaType followed by the 16-digit
// zero-padded sequence. Guarded by a Redis lock (if configured) plus a
// row-level lock on the counter row for defense in depth under concurrency.
//
// The width is 18, not 20, because of BCA's own field tables: its Payment
// (service 25) and Status (service 26) tables cap customerNo at String(18) and
// virtualAccountNo at String(26), while only the Inquiry table allows (20)/(28).
// A 20-digit customerNo yields a 28-character virtualAccountNo, which BCA's
// channel accepts on inquiry and then REJECTS on payment — the VA is
// inquirable but unpayable, which is the worst possible failure mode since the
// customer only discovers it at the point of paying. 16 digits of sequence is
// 10^16 VA numbers per type, so nothing is lost by fitting inside the narrower
// of the two limits.
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
		if len(seqStr) > 16 {
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

		customerNo = vaType + fmt.Sprintf("%016d", nextSeq)
		return nil
	})
	if err != nil {
		return "", err
	}
	return customerNo, nil
}

// RegisterStaticCustomerNo enforces that a merchant-supplied customerNo is
// only ever used once per partnerServiceId (feature 006-static-dynamic-va,
// FR-008). Returns domain.ErrVACustomerNoAlreadyRegistered on duplicate.
//
// Since feature 013-no-bill-payment-transaction this reads va_accounts rather
// than va_transactions: the registry is now where VA identity lives, and it
// carries a real UNIQUE (partner_service_id, customer_no) constraint backing
// this check. The Redis lock is retained so concurrent callers serialize
// rather than both observing "not yet registered".
//
// Only static BILL-bearing types still call this. No-bill types deliberately
// skip it, because a repeat /create-va on a registered no-bill VA is an update
// of the holder details, not a conflict (FR-005).
func (r *VARepository) RegisterStaticCustomerNo(ctx context.Context, partnerServiceID, customerNo string) error {
	return r.withLock(ctx, fmt.Sprintf("va-static-lock:%s:%s", partnerServiceID, customerNo), func() error {
		var existingCount int
		err := r.pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM va_accounts WHERE partner_service_id = $1 AND customer_no = $2`,
			partnerServiceID, customerNo,
		).Scan(&existingCount)
		if err != nil {
			return fmt.Errorf("customerNo registry unavailable: %w", err)
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
// FindVAInstalment implements domain.VARepository.
func (r *VARepository) FindVAInstalment(ctx context.Context, paymentRequestID string) (string, string, bool, error) {
	if paymentRequestID == "" {
		return "", "", false, nil
	}

	var transactionID string
	err := r.pool.QueryRow(ctx,
		`SELECT transaction_id FROM va_payments WHERE payment_request_id = $1`,
		paymentRequestID,
	).Scan(&transactionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, err
	}

	var cumulative string
	if err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount), 0)::text FROM va_payments WHERE transaction_id = $1`,
		transactionID,
	).Scan(&cumulative); err != nil {
		return "", "", false, err
	}
	return transactionID, cumulative, true, nil
}

func (r *VARepository) SaveVAPayment(ctx context.Context, transactionID, paymentRequestID, amount, referenceNo string) (paidAmount string, status string, recorded bool, err error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", "", false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// ON CONFLICT DO NOTHING on paymentRequestId is what stops a retried or
	// double-flagged instalment from being credited twice. recorded=false
	// tells the caller this payment was already on file, so it should replay
	// the outcome rather than notify the merchant again.
	tag, execErr := tx.Exec(ctx,
		`INSERT INTO va_payments (id, transaction_id, payment_request_id, amount, reference_no)
		 VALUES ($1, $2, NULLIF($3, ''), $4, $5)
		 ON CONFLICT (payment_request_id) WHERE payment_request_id IS NOT NULL DO NOTHING`,
		uuid.New().String(), transactionID, paymentRequestID, amount, referenceNo,
	)
	if execErr != nil {
		return "", "", false, execErr
	}
	recorded = tag.RowsAffected() > 0

	var totalAmount string
	err = tx.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount), 0)::text FROM va_payments WHERE transaction_id = $1`,
		transactionID,
	).Scan(&paidAmount)
	if err != nil {
		return "", "", false, err
	}

	err = tx.QueryRow(ctx,
		`SELECT total_amount::text FROM va_transactions WHERE id = $1`,
		transactionID,
	).Scan(&totalAmount)
	if err != nil {
		return "", "", false, err
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
		return "", "", false, err
	}

	if err = tx.Commit(ctx); err != nil {
		return "", "", false, err
	}

	return paidAmount, status, recorded, nil
}

// VA registry persistence (feature 013-no-bill-payment-transaction).
//
// These methods back va_accounts, the table holding VA identity that was
// previously conflated into va_transactions. As with this file's other
// SQL-heavy methods, live-database behavior is exercised by quickstart.md's
// integration scenarios rather than by unit tests here.

// vaAccountColumns is the shared SELECT list for va_accounts reads, kept in
// one place so GetVAAccount and GetVAAccountByPartnerAndCustomer cannot drift
// apart in column order.
const vaAccountColumns = `id, partner_service_id, customer_no, virtual_account_no, va_type, billing,
	customer_name, COALESCE(customer_email, ''), COALESCE(customer_phone, ''),
	trx_id, COALESCE(notification_url, ''), status, expired_date, created_at, updated_at`

// scanVAAccount reads one va_accounts row in vaAccountColumns order.
func scanVAAccount(row pgx.Row) (*domain.VAAccount, error) {
	account := &domain.VAAccount{}
	err := row.Scan(
		&account.ID,
		&account.PartnerServiceID,
		&account.CustomerNo,
		&account.VirtualAccountNo,
		&account.VAType,
		&account.Billing,
		&account.CustomerName,
		&account.CustomerEmail,
		&account.CustomerPhone,
		&account.TrxID,
		&account.NotificationURL,
		&account.Status,
		&account.ExpiredDate,
		&account.CreatedAt,
		&account.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrVAAccountNotFound
	}
	if err != nil {
		return nil, err
	}
	return account, nil
}

// SaveVAAccount upserts the VA registration keyed on virtual_account_no
// (feature 013-no-bill-payment-transaction, FR-002/FR-005). A repeat
// /create-va on an already-registered no-bill VA updates the holder details
// and reactivates the registration rather than conflicting.
//
// RETURNING id is scanned back for the same reason SaveInquiry does it: on the
// ON CONFLICT path the row keeps its ORIGINAL id, not the freshly generated
// one passed in, so callers must not trust account.ID as-is.
func (r *VARepository) SaveVAAccount(ctx context.Context, account *domain.VAAccount) error {
	if account.ID == "" {
		account.ID = uuid.New().String()
	}
	if account.CreatedAt.IsZero() {
		account.CreatedAt = time.Now()
	}
	account.UpdatedAt = time.Now()
	if account.Status == "" {
		account.Status = domain.VAAccountStatusActive
	}

	query := `
		INSERT INTO va_accounts (id, partner_service_id, customer_no, virtual_account_no, va_type, billing,
			customer_name, customer_email, customer_phone, trx_id, notification_url, status,
			expired_date, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (virtual_account_no) DO UPDATE SET
			customer_name = EXCLUDED.customer_name,
			customer_email = EXCLUDED.customer_email,
			customer_phone = EXCLUDED.customer_phone,
			trx_id = EXCLUDED.trx_id,
			notification_url = EXCLUDED.notification_url,
			expired_date = EXCLUDED.expired_date,
			status = EXCLUDED.status,
			updated_at = EXCLUDED.updated_at
		RETURNING id`

	return r.pool.QueryRow(ctx, query,
		account.ID,
		account.PartnerServiceID,
		account.CustomerNo,
		account.VirtualAccountNo,
		account.VAType,
		account.Billing,
		account.CustomerName,
		account.CustomerEmail,
		account.CustomerPhone,
		account.TrxID,
		account.NotificationURL,
		account.Status,
		account.ExpiredDate,
		account.CreatedAt,
		account.UpdatedAt,
	).Scan(&account.ID)
}

// GetVAAccount resolves the registration for a virtual account number. Unlike
// GetVAByVirtualAccountNo, this is an exact point read against a unique index
// — no ORDER BY ... LIMIT 1 heuristic is needed, because exactly one
// registration can exist per VA number.
//
// Returns domain.ErrVAAccountNotFound only for a genuine missing row; a
// failing query is returned verbatim so callers can tell "no registration,
// fall through to the legacy path" from "the database is broken".
func (r *VARepository) GetVAAccount(ctx context.Context, virtualAccountNo string) (*domain.VAAccount, error) {
	query := `SELECT ` + vaAccountColumns + ` FROM va_accounts WHERE virtual_account_no = $1`
	return scanVAAccount(r.pool.QueryRow(ctx, query, virtualAccountNo))
}

// GetVAAccountByPartnerAndCustomer resolves the registration by its
// (partnerServiceId, customerNo) identity rather than by VA number.
func (r *VARepository) GetVAAccountByPartnerAndCustomer(ctx context.Context, partnerServiceID, customerNo string) (*domain.VAAccount, error) {
	query := `SELECT ` + vaAccountColumns + ` FROM va_accounts WHERE partner_service_id = $1 AND customer_no = $2`
	return scanVAAccount(r.pool.QueryRow(ctx, query, partnerServiceID, customerNo))
}

// UpdateVAAccountStatus transitions a registration out of ACTIVE, returning
// domain.ErrVAAccountNotFound when no ACTIVE row matched.
//
// The "AND status = 'ACTIVE'" guard is load-bearing, not defensive: it is what
// makes the expiry callback exactly-once. Whichever concurrent inquiry or
// payment first detects the expiry applies the transition and gets a non-zero
// row count; every later caller gets zero rows and therefore knows not to
// enqueue a duplicate notification. This mirrors UpdateVAStatus's
// "WHERE status = '03'" guard on va_transactions.
func (r *VARepository) UpdateVAAccountStatus(ctx context.Context, virtualAccountNo string, status string) error {
	query := `
		UPDATE va_accounts
		SET status = $2, updated_at = NOW()
		WHERE virtual_account_no = $1 AND status = $3`

	result, err := r.pool.Exec(ctx, query, virtualAccountNo, status, domain.VAAccountStatusActive)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return domain.ErrVAAccountNotFound
	}
	return nil
}

// SaveNoBillPayment records ONE payment against a no-bill VA as its own
// va_transactions row (feature 013-no-bill-payment-transaction, FR-008).
//
// This is a plain INSERT, deliberately NOT the upsert SavePayment uses. A
// no-bill VA has no pending transaction to settle — every payment is a new
// transaction, which is exactly what lets the same VA number be paid an
// unlimited number of times. The caller sets InquiryRequestID to the
// paymentRequestId, so the existing UNIQUE index on inquiry_request_id makes a
// duplicate payment collide here instead of silently overwriting a settled
// row; that collision surfaces as domain.ErrVAPaymentDuplicate and the caller
// replays the original response.
func (r *VARepository) SaveNoBillPayment(ctx context.Context, payment *domain.VAPaymentRecord) error {
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
			free_texts, va_type, transaction_date, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21,
			$22, $23, $24, $25, $26, $27, $28, $29, $30)`

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
		payment.TotalAmount,
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
		payment.VAType,
		payment.TransactionDate,
		payment.CreatedAt,
		payment.UpdatedAt,
	)
	if isUniqueViolation(err) {
		return domain.ErrVAPaymentDuplicate
	}
	return err
}

// ListVAAccounts returns registered VA numbers — one row per VA, with that
// VA's settled-transaction count and total paid alongside (feature
// 013-no-bill-payment-transaction, FR-023).
//
// This replaces the merchant dashboard's old habit of listing va_transactions
// directly, under which a no-bill VA paid ten times rendered as ten separate
// VAs.
func (r *VARepository) ListVAAccounts(ctx context.Context, filter *domain.VAAccountListFilter) ([]domain.VAAccountListItem, int, error) {
	where := whereAlwaysTrue
	args := []interface{}{}
	argIdx := 1

	appendFilter := func(clause string, value interface{}) {
		where += fmt.Sprintf(" AND %s $%d", clause, argIdx)
		args = append(args, value)
		argIdx++
	}

	if filter.PartnerServiceID != "" {
		appendFilter("a.partner_service_id =", filter.PartnerServiceID)
	}
	if filter.FromDate != nil {
		appendFilter("a.created_at >=", *filter.FromDate)
	}
	if filter.ToDate != nil {
		appendFilter("a.created_at <=", *filter.ToDate)
	}
	if filter.Status != "" {
		appendFilter("a.status =", filter.Status)
	}
	if filter.VirtualAccountNo != "" {
		appendFilter("a.virtual_account_no =", filter.VirtualAccountNo)
	}

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM va_accounts a "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// The aggregate is scoped to settled ("00") transactions so pending or
	// deleted rows don't inflate a merchant's reported top-up total.
	dataQuery := `
		SELECT a.virtual_account_no, a.customer_no, a.customer_name, a.va_type, a.status,
			a.expired_date, a.created_at,
			COALESCE(agg.txn_count, 0), COALESCE(agg.total_paid, 0)::text
		FROM va_accounts a
		LEFT JOIN (
			SELECT virtual_account_no, COUNT(*) AS txn_count, SUM(paid_amount) AS total_paid
			FROM va_transactions
			WHERE status = '00'
			GROUP BY virtual_account_no
		) agg ON agg.virtual_account_no = a.virtual_account_no ` + where + `
		ORDER BY a.created_at DESC
		LIMIT $` + fmt.Sprintf("%d", argIdx) + ` OFFSET $` + fmt.Sprintf("%d", argIdx+1)

	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.pool.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []domain.VAAccountListItem
	for rows.Next() {
		var item domain.VAAccountListItem
		var totalPaid string
		if err := rows.Scan(
			&item.VirtualAccountNo,
			&item.CustomerNo,
			&item.CustomerName,
			&item.VAType,
			&item.Status,
			&item.ExpiredDate,
			&item.CreatedAt,
			&item.TransactionCount,
			&totalPaid,
		); err != nil {
			return nil, 0, err
		}
		item.TotalPaid = &domain.Amount{Value: totalPaid, Currency: "IDR"}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// ListVATransactions returns individual payment/transaction events — the
// per-payment view that complements ListVAAccounts' per-VA view (feature
// 013-no-bill-payment-transaction, FR-023). This is the query the merchant
// dashboard's list endpoint used to run directly; its status semantics
// ("00"/"02"/"03"/"04") are unchanged.
func (r *VARepository) ListVATransactions(ctx context.Context, filter *domain.VAListFilter) ([]domain.VATransactionListItem, int, error) {
	where := whereAlwaysTrue
	args := []interface{}{}
	argIdx := 1

	appendFilter := func(clause string, value interface{}) {
		where += fmt.Sprintf(" AND %s $%d", clause, argIdx)
		args = append(args, value)
		argIdx++
	}

	if filter.PartnerServiceID != "" {
		appendFilter("partner_service_id =", filter.PartnerServiceID)
	}
	if filter.FromDate != nil {
		appendFilter("created_at >=", *filter.FromDate)
	}
	if filter.ToDate != nil {
		appendFilter("created_at <=", *filter.ToDate)
	}
	if filter.Status != "" {
		appendFilter("status =", filter.Status)
	}
	if filter.VirtualAccountNo != "" {
		appendFilter("virtual_account_no =", filter.VirtualAccountNo)
	}

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM va_transactions "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	dataQuery := `
		SELECT virtual_account_no, customer_no, customer_name, COALESCE(payment_request_id, ''),
			COALESCE(reference_no, ''), paid_amount, total_amount, status, transaction_date, created_at
		FROM va_transactions ` + where + `
		ORDER BY created_at DESC
		LIMIT $` + fmt.Sprintf("%d", argIdx) + ` OFFSET $` + fmt.Sprintf("%d", argIdx+1)

	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.pool.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []domain.VATransactionListItem
	for rows.Next() {
		var item domain.VATransactionListItem
		var paidAmount, totalAmount *string
		var transactionDate *time.Time
		if err := rows.Scan(
			&item.VirtualAccountNo,
			&item.CustomerNo,
			&item.CustomerName,
			&item.PaymentRequestID,
			&item.ReferenceNo,
			&paidAmount,
			&totalAmount,
			&item.Status,
			&transactionDate,
			&item.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		if paidAmount != nil {
			item.PaidAmount = &domain.Amount{Value: *paidAmount, Currency: "IDR"}
		}
		if totalAmount != nil {
			item.TotalAmount = &domain.Amount{Value: *totalAmount, Currency: "IDR"}
		}
		item.TransactionDate = transactionDate
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return items, total, nil
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

// Vendor status reconciliation persistence
// (feature 014-vendor-status-reconciliation).

// ListStalePendingTransactions implements domain.StaleTransactionFinder.
//
// "Stale" is created_at < olderThan, not updated_at: updated_at moves on every
// upsert touch, so a VA that is repeatedly inquired would keep resetting its
// own clock and never become eligible — the exact transaction most likely to
// be stuck would be the one the sweep never looked at.
//
// Oldest first, so a backlog drains in the order the money went missing rather
// than the order Postgres happens to return.
func (r *VARepository) ListStalePendingTransactions(ctx context.Context, olderThan time.Time, limit int) ([]*domain.VAInquiryRecord, error) {
	if limit <= 0 {
		return nil, nil
	}

	query := `
		SELECT id, partner_service_id, customer_no, customer_name, virtual_account_no,
			inquiry_request_id, trx_id, notification_url, status, total_amount, currency,
			COALESCE(va_type, ''), COALESCE(sub_company, ''), free_texts, expired_date, created_at, updated_at
		FROM va_transactions
		WHERE status = '03'
		  AND created_at < $1
		  -- An expired VA is not stuck, it is closed: the customer can no
		  -- longer pay it, so there is nothing at the vendor to reconcile.
		  AND (expired_date IS NULL OR expired_date > NOW())
		ORDER BY created_at ASC
		LIMIT $2`

	rows, err := r.pool.Query(ctx, query, olderThan, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*domain.VAInquiryRecord
	for rows.Next() {
		record := &domain.VAInquiryRecord{}
		var freeTexts []byte
		if err := rows.Scan(
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
			&freeTexts,
			&record.ExpiredDate,
			&record.CreatedAt,
			&record.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if len(freeTexts) > 0 {
			_ = json.Unmarshal(freeTexts, &record.FreeTexts)
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

// CreateStatusInquiryAttempt implements domain.VAStatusInquiryRepository.
func (r *VARepository) CreateStatusInquiryAttempt(ctx context.Context, attempt *domain.VAStatusInquiryAttempt) error {
	if attempt.ID == "" {
		attempt.ID = uuid.New().String()
	}
	if attempt.AttemptedAt.IsZero() {
		attempt.AttemptedAt = time.Now()
	}

	// The nullable columns take NULL rather than "" so a query for "attempts
	// where the vendor answered" is a plain IS NOT NULL.
	query := `
		INSERT INTO va_status_inquiry_attempts (id, virtual_account_no, client_id, payment_request_id,
			outcome, bca_response_code, bca_payment_flag_status, duration_ms, error_detail, attempted_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	_, err := r.pool.Exec(ctx, query,
		attempt.ID,
		attempt.VirtualAccountNo,
		attempt.ClientID,
		nullIfEmpty(attempt.PaymentRequestID),
		attempt.Outcome,
		nullIfEmpty(attempt.VendorResponseCode),
		nullIfEmpty(attempt.VendorPaymentFlagStatus),
		attempt.DurationMs,
		nullIfEmpty(attempt.ErrorDetail),
		attempt.AttemptedAt,
	)
	return err
}

// nullIfEmpty maps "" to a SQL NULL.
func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Ensure VARepository also satisfies the reconciliation interfaces.
var _ domain.StaleTransactionFinder = (*VARepository)(nil)
var _ domain.VAStatusInquiryRepository = (*VARepository)(nil)

// SettlePendingFromVendor records a payment the vendor confirmed for a
// transaction this service still had pending, and reports whether THIS call is
// the one that applied it.
//
// The WHERE status = '03' guard is what makes reconciliation safe to run
// concurrently with a real /payment: the transition is a single atomic
// conditional write, so exactly one caller can win. A loser gets settled=false
// and must not send a second merchant callback — the money is recorded once,
// and the merchant is told once.
//
// Deliberately narrower than SavePayment's upsert: only the fields the vendor
// actually reported on a status inquiry are written. A status response carries
// no channelCode, journalNum or holder details, and blanking those over a real
// payment's values would lose information rather than add it.
func (r *VARepository) SettlePendingFromVendor(
	ctx context.Context,
	virtualAccountNo string,
	settlement domain.VendorSettlement,
) (bool, error) {
	query := `
		UPDATE va_transactions SET
			status = '00',
			paid_amount = $2,
			total_amount = COALESCE(total_amount, $2),
			currency = COALESCE(NULLIF(currency, ''), $3),
			payment_request_id = COALESCE(NULLIF($4, ''), payment_request_id),
			reference_no = COALESCE(NULLIF($5, ''), reference_no),
			transaction_date = $6,
			updated_at = NOW()
		WHERE virtual_account_no = $1 AND status = '03'`

	tag, err := r.pool.Exec(ctx, query,
		virtualAccountNo,
		settlement.PaidAmount,
		settlement.Currency,
		settlement.PaymentRequestID,
		settlement.ReferenceNo,
		settlement.TransactionDate,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// Ensure VARepository satisfies the reconciler's persistence needs.
var _ domain.ReconciliationRepository = (*VARepository)(nil)
