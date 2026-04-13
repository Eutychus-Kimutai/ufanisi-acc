package transport

import (
	"net/http"

	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/domain"
)

func NewRouter(ledger *domain.LedgerService) *http.ServeMux {
	router := http.NewServeMux()

	handler := NewHandler(ledger)
	router.HandleFunc("GET /accounts/{id}/transactions", handler.getTransactionsHandler)
	router.HandleFunc("GET /accounts/{id}", handler.getAccountHandler)
	router.HandleFunc("POST /accounts", handler.createAccountHandler)
	router.HandleFunc("POST /transactions", handler.transactionsHandler)
	return router
}
