package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/payment"
	"io"
)

func (h *Handler) HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Payment service is healthy"))
}
func (h *Handler) HandlePaymentEvent(w http.ResponseWriter, r *http.Request) {
	var event payment.PaymentEvent
	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read request body: %v", err), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	err = json.Unmarshal(data, &event)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse payment event: %v", err), http.StatusBadRequest)
		return
	}
	err = h.HandlePayment(event)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to handle payment event: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Payment event processed successfully"))
}
