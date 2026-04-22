package payment

type PaymentChannel string
type DestinationAccount string

const (
	mpesa DestinationAccount = "mpesa"
	bank  DestinationAccount = "bank"
	loan  DestinationAccount = "loan"
)

type PaymentEvent struct {
	ExternalId         string             `json:"external_id"`
	Amount             int64              `json:"amount"`
	PaymentChannel     PaymentChannel     `json:"payment_channel"`
	Destination        DestinationAccount `json:"destination"`
	ClientRef          string             `json:"reference"`
	AccountReference  string             `json:"account_reference"`
	PhoneNumber       string             `json:"phone_number"`
}
