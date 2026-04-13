package transport

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/database"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/domain"
	"github.com/google/uuid"
)

type Handler struct {
	ledger *domain.LedgerService
}

func NewHandler(ledger *domain.LedgerService) *Handler {
	return &Handler{ledger: ledger}
}

func (h *Handler) getAccountHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "Missing account ID", http.StatusBadRequest)
		return
	}

	acc, err := h.ledger.GetAccount(context.Background(), id)
	if err != nil {
		http.Error(w, "Account not found", http.StatusNotFound)
		return
	}
	balance, err := h.ledger.GetBalance(context.Background(), acc.ID)
	if err != nil {
		http.Error(w, "Failed to get account balance", http.StatusInternalServerError)
		return
	}

	response, err := json.Marshal(struct {
		Account database.Account `json:"account"`
		Balance int64            `json:"balance"`
	}{
		Account: acc,
		Balance: balance,
	})
	if err != nil {
		http.Error(w, "Failed to marshal account", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(response)
}

func (h *Handler) createAccountHandler(w http.ResponseWriter, r *http.Request) {
	type request struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	id := uuid.New()
	var req request
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	log.Printf("Received create account request: %+v\n", req)
	err := h.ledger.CreateAccount(context.Background(), database.Account{
		ID:   id,
		Name: req.Name,
		Type: req.Type,
	})
	if err != nil {
		log.Printf("Failed to create account: %v\n", err)
		return
	}

	response, err := json.Marshal(req)
	if err != nil {
		http.Error(w, "Failed to marshal account", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write(response)
}
func (h *Handler) transactionsHandler(w http.ResponseWriter, r *http.Request) {
	type request struct {
		Reference string `json:"reference"`
		Entries   []struct {
			TransactionId uuid.UUID `json:"transaction_id"`
			AccountId     uuid.UUID `json:"account_id"`
			Amount        int64     `json:"amount"`
			Type          string    `json:"type"`
		} `json:"entries"`
	}

	id := uuid.New()
	var entries []domain.Entry
	var req request
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	for _, e := range req.Entries {
		entries = append(entries, domain.Entry{
			TransactionId: id,
			AccountId:     e.AccountId,
			Amount:        e.Amount,
			Type:          domain.EntryType(e.Type),
		})

	}
	err := h.ledger.PostTransaction(context.Background(), domain.Transaction{
		Id:        id,
		Reference: req.Reference,
		Entries:   entries,
	})
	if err != nil {
		log.Printf("Failed to post transaction: %v\n", err)
		http.Error(w, "Failed to post transaction", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(`{"message": "Transaction posted successfully"}`))
}

func (h *Handler) getTransactionsHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "Missing account ID", http.StatusBadRequest)
		return
	}

	transactions, err := h.ledger.GetAccountHistory(context.Background(), id)
	if err != nil {
		http.Error(w, "Failed to get transactions", http.StatusInternalServerError)
		return
	}

	response, err := json.Marshal(transactions)
	if err != nil {
		http.Error(w, "Failed to marshal transactions", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(response)
}
