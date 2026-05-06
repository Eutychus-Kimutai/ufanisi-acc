package payment

type PaymentChannel string
type DestinationAccount string

const (
	DestinationAccountLoan       DestinationAccount = "loan"
	DestinationAccountInvestment DestinationAccount = "investment"
)

type PaymentEvent struct {
	ExternalId       string             `json:"external_id"`
	Amount           int64              `json:"amount"`
	PaymentChannel   PaymentChannel     `json:"payment_channel"`
	Destination      DestinationAccount `json:"destination"`
	ClientRef        string             `json:"reference"`
	AccountReference string             `json:"account_reference"`
	PhoneNumber      string             `json:"phone_number"`
}
