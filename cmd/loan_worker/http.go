package loanworker

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/payment"
)

type HTTPHandler struct {
	worker *Worker
}

func NewHTTPHandler(w *Worker) *HTTPHandler {
	return &HTTPHandler{
		worker: w,
	}
}

func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/payment":
		h.handlePaymentEvent(w, r)
	case "/health":
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	default:
		http.NotFound(w, r)
	}
}

func (h *HTTPHandler) handlePaymentEvent(w http.ResponseWriter, r *http.Request) {
	var event payment.PaymentEvent
	err := json.NewDecoder(r.Body).Decode(&event)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err = h.worker.HandlePaymentEvent(r.Context(), event)
	if err != nil {
		log.Printf("Error handling payment event: %v\n", err)
		http.Error(w, "Failed to process payment event", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}
