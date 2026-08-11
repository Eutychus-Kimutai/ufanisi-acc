package commands

import (
	"encoding/json"
	"github.com/google/uuid"
	"log"
	"time"
)

type CommandType string

const (
	PostTransaction               CommandType = "POST_TRANSACTION"
	ApplyLoanRepayment            CommandType = "APPLY_LOAN_REPAYMENT"
	UnresolvedPayment             CommandType = "UNRESOLVED_PAYMENT"
	PaymentCompleted              CommandType = "PAYMENT_COMPLETED"
	InvestmentAccrued             CommandType = "INVESTMENT_ACCRUED"
	InvestmentWithdrawalRequested CommandType = "INVESTMENT_WITHDRAWAL_REQUESTED"
	InvestmentWithdrawalProcessed CommandType = "INVESTMENT_WITHDRAWAL_PROCESSED"
	InvestmentMatured             CommandType = "INVESTMENT_MATURED"
	ResolvePayment                CommandType = "RESOLVE_PAYMENT"
	PaymentResolved               CommandType = "PAYMENT_RESOLVED"
)

type Entry struct {
	TransactionID string `json:"transaction_id"`
	AccountID     string `json:"account_id"`
	Amount        int64  `json:"amount"`
	Type          string `json:"type"`
}

type PaymentResolvedPayload struct {
	IdempotencyKey string    `json:"payment_id"`
	ResolvedAt     time.Time `json:"resolved_at"`
}
type ResolvePaymentPayload struct {
	PaymentID   uuid.UUID `json:"payment_id"`
	PaymentRef  string    `json:"payment_ref"`
	ClientRef   string    `json:"client_ref"`
	Amount      int64     `json:"amount"`
	ExternalId  string    `json:"external_id"`
	AccountRef  string    `json:"account_ref"`
	PhoneNumber string    `json:"phone_number"`
}
type PaymentCompletedPayload struct {
	IdempotencyKey string `json:"idempotency_key"`
	ExternalId     string `json:"external_id"`
	Amount         int64  `json:"amount"`
	PaymentType    string `json:"payment_channel"`
	PhoneNumber    string `json:"phone_number"`
	ClientRef      string `json:"client_ref"`
	AccountRef     string `json:"account_ref"`
	Destination    string `json:"destination"`
	RawEvent       string `json:"raw_event"`
}

type LoanRepaymentPayload struct {
	ClientID       string `json:"client_id"`
	Amount         int64  `json:"amount"`
	ReferenceID    string `json:"reference_id"`
	PaymentChannel string `json:"payment_channel"`
	Reference      string `json:"reference"`
}

type UnresolvedPaymentPayload struct {
	ClientRef   string `json:"client_ref"`
	Amount      int64  `json:"amount"`
	PaymentType string `json:"payment_channel"`
	ExternalId  string `json:"external_id"`
	Reason      string `json:"reason"`
}

type InvestmentAccruedPayload struct {
	InvestmentId    string `json:"investment_id"`
	AccrualAmount   int64  `json:"accrual_amount"`
	NewAccruedTotal int64  `json:"new_accrued_total"`
	NextAccrualDate string `json:"next_accrual_date"`
}

type InvestmentWithdrawalRequestedPayload struct {
	InvestmentId string `json:"investment_id"`
	WithdrawalId string `json:"withdrawal_id"`
	Amount       int64  `json:"amount"`
	Status       string `json:"status"`
	RequestedAt  string `json:"requested_at"`
	EligibleAt   string `json:"eligible_at"`
}

type InvestmentWithdrawalProcessedPayload struct {
	InvestmentId string `json:"investment_id"`
	Amount       int64  `json:"amount"`
}

type AccrualNoticePayload struct {
	InvestmentId  string `json:"investment_id"`
	AccrualAmount int64  `json:"accrual_amount"`
}

type InvestmentMaturedPayload struct {
	InvestmentId string `json:"investment_id"`
}
type Payload struct {
	Reference string  `json:"reference"`
	Entries   []Entry `json:"entries"`
}
type Command struct {
	Type    CommandType     `json:"command_type"`
	Payload json.RawMessage `json:"payload"`
}

func NewCommand(cmdType CommandType, payload interface{}) (Command, error) {
	return Command{
		Type:    cmdType,
		Payload: marshalPayload(payload),
	}, nil
}

func marshalPayload(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		log.Fatalf("Failed to marshal payload: %v", err)
		return nil
	}
	return json.RawMessage(data)
}
