package transport

import (
	"database/sql"
	"net/http"

	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/domain"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/rabbitmq"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/repository"
)

func NewRouter(db *sql.DB, ledger *domain.LedgerService, investment *repository.InvestmentRepository, publisher rabbitmq.Publisher) *http.ServeMux {
	router := http.NewServeMux()

	handler := NewHandler(ledger, investment, publisher)
	router.HandleFunc("GET /accounts/{id}/transactions", handler.getTransactionsHandler)
	router.HandleFunc("GET /accounts/{id}", handler.getAccountHandler)
	router.HandleFunc("POST /accounts", handler.createAccountHandler)
	router.HandleFunc("POST /transactions", handler.transactionsHandler)
	router.HandleFunc("POST /investments", handler.createInvestmentHandler)
	return router
}
