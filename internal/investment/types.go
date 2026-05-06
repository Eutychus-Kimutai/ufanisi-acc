package investment

import (
	"time"

	"github.com/google/uuid"
)

type Investment struct {
	Id              string  `json:"id"`
	ClientId        string  `json:"client_id"`
	Principal       float64 `json:"principal"`
	AnnualRate      float64 `json:"annual_rate"`
	Status          string  `json:"status"`
	AccruedInterest float64 `json:"accrued_interest"`
	NextAccrualDate string  `json:"next_accrual_date"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

type WithdrawalRequest struct {
	Id          string           `json:"id"`
	ClientId    string           `json:"client_id"`
	Amount      float64          `json:"amount"`
	Status      WithdrawalStatus `json:"status"`
	RequestedAt time.Time        `json:"requested_at"`
	EligibleAt  time.Time        `json:"eligible_at"`
	CreatedAt   time.Time        `json:"created_at"`
}

type Accrual struct {
	Id           string    `json:"id"`
	InvestmentId uuid.UUID `json:"investment_id"`
	PeriodStart  time.Time `json:"period_start"`
	PeriodEnd    time.Time `json:"period_end"`
	Interest     float64   `json:"interest"`
	CreatedAt    time.Time `json:"created_at"`
}

type WithdrawalStatus string

const (
	Pending     WithdrawalStatus = "pending"
	Eligible    WithdrawalStatus = "eligible"
	Completed   WithdrawalStatus = "completed"
	NoticeGiven WithdrawalStatus = "notice_given"
)
