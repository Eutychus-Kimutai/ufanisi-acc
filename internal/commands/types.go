package commands

import (
	"encoding/json"
	"log"
)

type CommandType string

const (
	PostTransaction               CommandType = "POST_TRANSACTION"
	ApplyLoanRepayment            CommandType = "APPLY_LOAN_REPAYMENT"
	UnresolvedPayment             CommandType = "UNRESOLVED_PAYMENT"
	InvestmentCreated             CommandType = "INVESTMENT_CREATED"
	InvestmentAccrued             CommandType = "INVESTMENT_ACCRUED"
	InvestmentWithdrawalRequested CommandType = "INVESTMENT_WITHDRAWAL_REQUESTED"
	InvestmentWithdrawalProcessed CommandType = "INVESTMENT_WITHDRAWAL_PROCESSED"
	InvestmentMatured             CommandType = "INVESTMENT_MATURED"
)

type Entry struct {
	AccountID string `json:"account_id"`
	Amount    int64  `json:"amount"`
	Type      string `json:"type"`
}

type LoanRepaymentPayload struct {
	ClientID       string `json:"client_id"`
	Amount         int64  `json:"amount"`
	ReferenceID    string `json:"reference_id"`
	PaymentChannel string `json:"payment_channel"`
	Reference      string `json:"reference"`
}

type UnresolvedPaymentPayload struct {
	ClientRef      string `json:"client_ref"`
	Amount         int64  `json:"amount"`
	PaymentChannel string `json:"payment_channel"`
	ExternalId     string `json:"external_id"`
	Reason         string `json:"reason"`
}

type InvestmentCreatedPayload struct {
	Id              string  `json:"id"`
	ClientId        string  `json:"client_id"`
	Principal       int64   `json:"principal"`
	AnnualRate      float64 `json:"annual_rate"`
	Status          string  `json:"status"`
	AccruedInterest int64   `json:"accrued_interest"`
	NextAccrualDate string  `json:"next_accrual_date"`
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
