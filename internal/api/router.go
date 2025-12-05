package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"tezos-delegation-service/internal/store"
)

type Server struct {
	store store.DelegationStore
	db    *sql.DB
}

func NewRouter(s store.DelegationStore, db *sql.DB) http.Handler {
	srv := &Server{store: s, db: db}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", srv.handleHealth)
	mux.HandleFunc("/xtz/delegations", srv.handleDelegations)

	handler := loggingMiddleware(mux)
	handler = recoveryMiddleware(handler)
	handler = corsMiddleware(handler)

	return handler
}

type responseDelegation struct {
	Timestamp string `json:"timestamp"`
	Amount    string `json:"amount"`
	Delegator string `json:"delegator"`
	Level     string `json:"level"`
}

type response struct {
	Data []responseDelegation `json:"data"`
}

type healthResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
	Uptime string            `json:"uptime,omitempty"`
}

var startTime = time.Now()

func (s *Server) handleDelegations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	yearParam := r.URL.Query().Get("year")
	var year *int
	if yearParam != "" {
		y, err := strconv.Atoi(yearParam)
		if err != nil || y < 2018 {
			http.Error(w, "invalid year", http.StatusBadRequest)
			return
		}
		year = &y
	}

	pageParam := r.URL.Query().Get("page")
	page := 1
	if pageParam != "" {
		p, err := strconv.Atoi(pageParam)
		if err != nil || p <= 0 {
			http.Error(w, "invalid page", http.StatusBadRequest)
			return
		}
		page = p
	}

	const pageSize = 50
	offset := (page - 1) * pageSize

	rows, err := s.store.GetPage(ctx, year, pageSize, offset)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	out := response{
		Data: make([]responseDelegation, 0, len(rows)),
	}
	for _, d := range rows {
		out.Data = append(out.Data, responseDelegation{
			Timestamp: d.Timestamp.UTC().Format("2006-01-02T15:04:05Z"),
			Amount:    strconv.FormatInt(d.Amount, 10),
			Delegator: d.Delegator,
			Level:     strconv.FormatInt(d.Level, 10),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	checks := make(map[string]string)
	status := "healthy"

	// Check database connection
	if err := s.db.PingContext(ctx); err != nil {
		checks["database"] = "unhealthy: " + err.Error()
		status = "unhealthy"
	} else {
		checks["database"] = "healthy"
	}

	// Check database stats
	stats := s.db.Stats()
	if stats.OpenConnections > 0 {
		checks["database_connections"] = strconv.Itoa(stats.OpenConnections) + " open"
	}

	resp := healthResponse{
		Status: status,
		Checks: checks,
		Uptime: time.Since(startTime).Round(time.Second).String(),
	}

	w.Header().Set("Content-Type", "application/json")
	if status != "healthy" {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// loggingMiddleware logs HTTP requests
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(lrw, r)

		log.Printf("%s %s %d %s",
			r.Method,
			r.URL.Path,
			lrw.statusCode,
			time.Since(start),
		)
	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// recoveryMiddleware recovers from panics and returns 500
func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic recovered: %v", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// corsMiddleware adds CORS headers
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
