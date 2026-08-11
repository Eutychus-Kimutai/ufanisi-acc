package transport

import (
	"context"

	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/commands"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/database"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/domain"
	invworker "github.com/Eutychus-Kimutai/ufanisi-acc/internal/ingestion/investment"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/rabbitmq"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/repository"
	"github.com/google/uuid"
)

type Handler struct {
	ledger     *domain.LedgerService
	investment *repository.InvestmentRepository
	publisher  rabbitmq.Publisher
	db         *sql.DB
	cfg        *rabbitmq.RabbitConfig

	invWorker  *invworker.Worker
	workerOnce sync.Once
	workerErr  error
}

func NewHandler(ledger *domain.LedgerService, investment *repository.InvestmentRepository, publisher rabbitmq.Publisher, db *sql.DB, cfg *rabbitmq.RabbitConfig) *Handler {
	return &Handler{ledger: ledger, investment: investment, publisher: publisher, db: db, cfg: cfg}
}

func (h *Handler) worker() (*invworker.Worker, error) {
	h.workerOnce.Do(func() {
		h.invWorker, h.workerErr = invworker.NewWorker(h.db, h.publisher, h.cfg, database.New(h.db))
	})
	return h.invWorker, h.workerErr
}
func (h *Handler) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "ok"}`))
}

func (h *Handler) getAccountHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing account ID", http.StatusBadRequest)
		return
	}

	acc, err := h.ledger.GetAccount(context.Background(), id)
	if err != nil {
		http.Error(w, "Account not found", http.StatusNotFound)
		return
	}
	balance, err := h.ledger.GetBalance(context.Background(), acc.Name)
	if err != nil {
		http.Error(w, "failed to get account balance", http.StatusInternalServerError)
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
		http.Error(w, "failed to marshal account", http.StatusInternalServerError)
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
		log.Printf("failed to create account: %v\n", err)
		return
	}

	response, err := json.Marshal(req)
	if err != nil {
		http.Error(w, "failed to marshal account", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write(response)
}
func (h *Handler) transactionsHandler(w http.ResponseWriter, r *http.Request) {
	type request struct {
		Type    string `json:"type"`
		Entries []struct {
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
		Id:      id,
		Type:    req.Type,
		Entries: entries,
	})
	if err != nil {
		log.Printf("failed to post transaction: %v\n", err)
		http.Error(w, "failed to post transaction", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(`{"message": "Transaction posted successfully"}`))
}

func (h *Handler) getTransactionsHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing account ID", http.StatusBadRequest)
		return
	}

	transactions, err := h.ledger.GetAccountHistory(context.Background(), id)
	if err != nil {
		http.Error(w, "failed to get transactions", http.StatusInternalServerError)
		return
	}

	response, err := json.Marshal(transactions)
	if err != nil {
		http.Error(w, "failed to marshal transactions", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(response)
}

func (h *Handler) createInvestmentHandler(w http.ResponseWriter, r *http.Request) {
	var req commands.ResolvePaymentPayload
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	worker, err := h.worker()
	if err != nil {
		log.Printf("failed to create investment worker: %v\n", err)
		http.Error(w, "failed to process investment", http.StatusInternalServerError)
		return
	}
	err = worker.HandlePaymentEvent(r.Context(), req)
	if err != nil {
		log.Printf("failed to handle payment event: %v\n", err)
		http.Error(w, "failed to create investment", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(`{"message": "investment created successfully"}`))
}

func (h *Handler) handleRequestWithdrawal(w http.ResponseWriter, r *http.Request) {
	type request struct {
		InvestmentID uuid.UUID `json:"investment_id"`
		Amount       int64     `json:"amount"`
	}
	var req request
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	log.Printf("Received withdrawal request: %+v\n", req)
	if req.Amount <= 0 {
		http.Error(w, "Withdrawal amount must be greater than zero", http.StatusBadRequest)
		return
	}
	worker, err := h.worker()
	if err != nil {
		log.Printf("failed to create investment worker: %v\n", err)
		http.Error(w, "failed to process withdrawal request", http.StatusInternalServerError)
		return
	}
	err = worker.RequestWithdrawal(r.Context(), req.InvestmentID, req.Amount, 1)
	if err != nil {
		log.Printf("failed to process withdrawal request: %v\n", err)
		http.Error(w, "failed to process withdrawal request", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message": "withdrawal request created successfully"}`))
}
