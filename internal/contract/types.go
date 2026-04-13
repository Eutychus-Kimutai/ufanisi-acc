package contract

import (
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/domain"
	"github.com/google/uuid"
)

type CreateAccountRequest struct {
	Name string             `json:"name"`
	Type domain.AccountType `json:"type"`
}
type CreateAccountResponse struct {
	Account domain.Account `json:"account"`
}
type PostTransactionRequest struct {
	Reference string       `json:"reference"`
	Entries   []EntryInput `json:"entries"`
}
type EntryInput struct {
	AccountId uuid.UUID        `json:"account_id"`
	Amount    int64            `json:"amount"`
	Type      domain.EntryType `json:"type"`
}
type CreateTransactionResponse struct {
	Transaction domain.Transaction `json:"transaction"`
}
type AccountResponse struct {
	Account domain.Account `json:"account"`
}
type ErrorResponse struct {
	Error string `json:"error"`
}
