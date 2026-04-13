package domain

import (
	"time"

	"github.com/google/uuid"
)

type Account struct {
	Id        uuid.UUID
	Name      string
	Type      AccountType
	CreatedAt time.Time
}

type Transaction struct {
	Id        uuid.UUID
	Reference string
	CreatedAt time.Time
	Entries   []Entry
}

type Entry struct {
	TransactionId uuid.UUID
	AccountId     uuid.UUID
	Amount        int64
	Type          EntryType
}

type AccountType string

const (
	Asset     AccountType = "asset"
	Liability AccountType = "liability"
	Equity    AccountType = "equity"
	Revenue   AccountType = "revenue"
	Expense   AccountType = "expense"
)

type EntryType string

const (
	Debit  EntryType = "Debit"
	Credit EntryType = "Credit"
)
