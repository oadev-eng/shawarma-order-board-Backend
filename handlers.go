package main

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// API bundles the store with its HTTP handlers.
type API struct {
	store *Store
}

func NewAPI(store *Store) *API {
	return &API{store: store}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// withCORS allows the frontend (served from a different origin, e.g. a
// claude.ai artifact, localhost:5173, or a static file host) to call this
// API directly.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// validateItems checks every id is on the menu, quantities aren't negative,
// and strips zero-quantity entries. Returns the cleaned map and the total
// item count.
func validateItems(items OrderItems) (OrderItems, int, error) {
	cleaned := OrderItems{}
	total := 0
	for id, qty := range items {
		if _, ok := MenuByID[id]; !ok {
			return nil, 0, errUnknownItem(id)
		}
		if qty < 0 {
			return nil, 0, errNegativeQty
		}
		if qty > 0 {
			cleaned[id] = qty
			total += qty
		}
	}
	return cleaned, total, nil
}

type apiError struct{ msg string }

func (e apiError) Error() string { return e.msg }
func errUnknownItem(id string) error {
	return apiError{"unknown menu item: " + id}
}

var errNegativeQty = apiError{"quantity cannot be negative"}

// GET /api/menu
func (a *API) handleMenu(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, Menu)
}

type orderRequest struct {
	Items   OrderItems `json:"items"`
	Note    string     `json:"note"`
	Source  string      `json:"source"`
	ForDate string     `json:"forDate"`
}

// POST /api/orders
func (a *API) handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	var req orderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	forDate := req.ForDate
	if forDate == "" {
		forDate = todayString()
	}
	if !isValidDate(forDate) {
		writeError(w, http.StatusBadRequest, "forDate must be YYYY-MM-DD")
		return
	}

	cleaned, total, err := validateItems(req.Items)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if total == 0 {
		writeError(w, http.StatusBadRequest, "order must include at least one item")
		return
	}

	o, err := a.store.Create(cleaned, req.Note, req.Source, forDate)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not save order")
		return
	}
	writeJSON(w, http.StatusCreated, o)
}

// PUT /api/orders/{id}  - edit a ticket that's still pending
func (a *API) handleUpdateOrder(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid order id")
		return
	}

	var req orderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	forDate := req.ForDate
	if forDate == "" {
		forDate = todayString()
	}
	if !isValidDate(forDate) {
		writeError(w, http.StatusBadRequest, "forDate must be YYYY-MM-DD")
		return
	}

	cleaned, total, err := validateItems(req.Items)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if total == 0 {
		writeError(w, http.StatusBadRequest, "order must include at least one item")
		return
	}

	o, err := a.store.Update(id, cleaned, req.Note, req.Source, forDate)
	if err != nil {
		writeError(w, http.StatusNotFound, "order not found")
		return
	}
	writeJSON(w, http.StatusOK, o)
}

// GET /api/orders?status=pending|done&date=YYYY-MM-DD  (either filter optional)
func (a *API) handleListOrders(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status != "" && status != "pending" && status != "done" {
		writeError(w, http.StatusBadRequest, "status must be 'pending' or 'done'")
		return
	}
	forDate := r.URL.Query().Get("date")
	if forDate != "" && !isValidDate(forDate) {
		writeError(w, http.StatusBadRequest, "date must be YYYY-MM-DD")
		return
	}
	writeJSON(w, http.StatusOK, a.store.List(status, forDate))
}

// POST /api/orders/{id}/complete
func (a *API) handleCompleteOrder(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid order id")
		return
	}
	o, err := a.store.Complete(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "order not found")
		return
	}
	writeJSON(w, http.StatusOK, o)
}

// POST /api/orders/{id}/reopen
func (a *API) handleReopenOrder(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid order id")
		return
	}
	o, err := a.store.Reopen(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "order not found")
		return
	}
	writeJSON(w, http.StatusOK, o)
}

// DELETE /api/orders/{id}
func (a *API) handleDeleteOrder(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid order id")
		return
	}
	if err := a.store.Delete(id); err != nil {
		writeError(w, http.StatusNotFound, "order not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/completed/clear?date=YYYY-MM-DD  (date optional - omit to clear all)
func (a *API) handleClearCompleted(w http.ResponseWriter, r *http.Request) {
	forDate := r.URL.Query().Get("date")
	if forDate != "" && !isValidDate(forDate) {
		writeError(w, http.StatusBadRequest, "date must be YYYY-MM-DD")
		return
	}
	n := a.store.ClearCompleted(forDate)
	writeJSON(w, http.StatusOK, map[string]int{"cleared": n})
}

// GET /api/summary?date=YYYY-MM-DD  (defaults to today if omitted)
func (a *API) handleSummary(w http.ResponseWriter, r *http.Request) {
	forDate := r.URL.Query().Get("date")
	if forDate == "" {
		forDate = todayString()
	}
	if !isValidDate(forDate) {
		writeError(w, http.StatusBadRequest, "date must be YYYY-MM-DD")
		return
	}
	writeJSON(w, http.StatusOK, a.store.Summary(forDate))
}
