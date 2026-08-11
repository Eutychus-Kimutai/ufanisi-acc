package payment

import "encoding/json"

type PaymentChannel string
type DestinationAccount string

const (
	DestinationAccountLoan       DestinationAccount = "loan"
	DestinationAccountInvestment DestinationAccount = "investment"
)

type PaymentEvent struct {
	ExternalID       string          `json:"external_id"`
	Amount           int64           `json:"amount"`
	PaymentType      string          `json:"payment_Type"`
	PaymentReference string          `json:"payment_ref"`
	PhoneNumber      string          `json:"phone_number"`
	RawEvent         json.RawMessage `json:"raw_event"`
}

type Payment struct {
	ExternalID       string             `json:"external_id"`
	Amount           int64              `json:"amount"`
	PaymentType      string             `json:"payment_type"`
	PaymentRef       string             `json:"payment_ref"`
	Destination      DestinationAccount `json:"destination"`
	ClientRef        string             `json:"reference"`
	AccountReference string             `json:"account_reference"`
	PhoneNumber      string             `json:"phone_number"`
	Status           string             `json:"status"`
	ResolvedType     string             `json:"resolved_type"`
	RawEvent         string             `json:"raw_event"`
}
