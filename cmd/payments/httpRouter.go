package main

import (
	"net/http"
)

func PaymentsRouter(h *Handler) *http.ServeMux {
	router := http.NewServeMux()
	router.HandleFunc("/health", h.HealthCheckHandler)
	router.HandleFunc("POST /payments", h.HandlePaymentEvent)
	return router
}
