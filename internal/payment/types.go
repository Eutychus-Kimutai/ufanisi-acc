package payment

type PaymentChannel string
type DestinationAccount string

const (
	DestinationAccountMpesa DestinationAccount = "mpesa"
	DestinationAccountBank  DestinationAccount = "bank"
	DestinationAccountLoan  DestinationAccount = "loan"
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
