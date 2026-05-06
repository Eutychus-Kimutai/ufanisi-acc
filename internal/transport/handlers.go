package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/commands"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/database"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/domain"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/rabbitmq"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/repository"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Handler struct {
	ledger       *domain.LedgerService
	investment   *repository.InvestmentRepository
	publisher    rabbitmq.Publisher
	capitalAccID uuid.UUID
}

type CreateInvestmentRequest struct {
	AccountID          uuid.UUID `json:"account_id"`
	ClientID           uuid.UUID `json:"client_id"`
	PrincipalInitial   int64     `json:"principal_initial"`
	AnnualRate         float64   `json:"annual_rate"`
	NoticePeriodMonths int       `json:"notice_period_months"`
}

type CreateInvestmentResponse struct {
	Investment Investment `json:"investment"`
}

type Investment struct {
	ID                 uuid.UUID `json:"id"`
	ClientID           uuid.UUID `json:"client_id"`
	PrincipalInitial   int64     `json:"principal_initial"`
	AnnualRate         float64   `json:"annual_rate"`
	NextAccrualAt      string    `json:"next_accrual_at"`
	NoticePeriodMonths int       `json:"notice_period_months"`
}

func NewHandler(ledger *domain.LedgerService, investment *repository.InvestmentRepository, publisher rabbitmq.Publisher) *Handler {
	capitalAccID, err := investment.GetInvestmentsCapitalAccount(context.Background())
	if err != nil {
		log.Fatalf("Failed to get investor capital account: %v", err)
	}
	return &Handler{ledger: ledger, investment: investment, publisher: publisher, capitalAccID: *capitalAccID}
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
		Balance float64          `json:"balance"`
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

func (h *Handler) createInvestmentHandler(w http.ResponseWriter, r *http.Request) {
	type request struct {
		AccountID          uuid.UUID `json:"account_id"`
		ClientID           uuid.UUID `json:"client_id"`
		PrincipalInitial   int64     `json:"principal_initial"`
		AnnualRate         float64   `json:"annual_rate"`
		NoticePeriodMonths int       `json:"notice_period_months"`
	}
	var req request
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	log.Printf("Received create investment request: %+v\n", req)

	inv, err := h.investment.CreateInvestment(context.Background(), database.Investment{
		ClientID:         req.ClientID,
		PrincipalInitial: req.PrincipalInitial,
	})
	if err != nil {
		log.Printf("Failed to create investment: %v\n", err)
		http.Error(w, "Failed to create investment", http.StatusInternalServerError)
		return
	}

	// Create funding transaction
	fundingTx := domain.Transaction{
		Reference: fmt.Sprintf("Investment_created_%s", inv.ID),
		Entries: []domain.Entry{
			{
				AccountId: req.AccountID,
				Amount:    req.PrincipalInitial,
				Type:      domain.Debit,
			},
			{
				AccountId: h.capitalAccID,
				Amount:    req.PrincipalInitial,
				Type:      domain.Credit,
			},
		},
	}
	err = h.ledger.PostTransaction(context.Background(), fundingTx)
	if err != nil {
		log.Printf("Failed to post funding transaction: %v\n", err)
		http.Error(w, "Failed to post funding transaction", http.StatusInternalServerError)
		return
	}
	rate, err := strconv.ParseFloat(inv.AnnualRate, 64)
	if err != nil {
		log.Printf("Failed to parse annual rate: %v\n", err)
		http.Error(w, "Failed to parse annual rate", http.StatusInternalServerError)
		return
	}
	response, err := json.Marshal(Investment{
		ID:               inv.ID,
		ClientID:         inv.ClientID,
		PrincipalInitial: inv.PrincipalInitial,
		NextAccrualAt:    inv.NextAccrualAt.Format("2006-01-02"),
		AnnualRate:       rate,
	})
	if err != nil {
		http.Error(w, "Failed to marshal investment", http.StatusInternalServerError)
		return
	}

	event := commands.InvestmentCreatedPayload{
		Id:              inv.ID.String(),
		ClientId:        inv.ClientID.String(),
		Principal:       inv.PrincipalInitial,
		Status:          inv.Status,
		AccruedInterest: inv.AccruedInterest,
		NextAccrualDate: inv.NextAccrualAt.Format("2006-01-02"),
		AnnualRate:      rate,
	}
	cmd, err := commands.NewCommand(commands.InvestmentCreated, event)
	if err != nil {
		log.Printf("Failed to create command: %v\n", err)
		http.Error(w, "Failed to create command", http.StatusInternalServerError)
		return
	}
	err = h.publisher.Publish("", string(commands.InvestmentCreated), false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        []byte(cmd.Payload),
	})
	if err != nil {
		log.Printf("Failed to publish command: %v\n", err)
		http.Error(w, "Failed to publish command", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write(response)
}
