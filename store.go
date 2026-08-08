package main

import (
	"encoding/json"
	"errors"
	"os"
	"sort"
	"sync"
	"time"
)

// ErrNotFound is returned when an order id doesn't exist.
var ErrNotFound = errors.New("order not found")

// snapshot is the on-disk shape of the store.
type snapshot struct {
	Orders []*Order `json:"orders"`
	NextID int      `json:"nextId"`
}

// Store holds all orders in memory and mirrors every change to a JSON file,
// so a restart of the server doesn't lose the day's (or any day's) tickets.
type Store struct {
	mu     sync.Mutex
	orders map[int]*Order
	nextID int
	path   string
}

// NewStore creates a store backed by the JSON file at path, loading any
// existing data found there.
func NewStore(path string) *Store {
	s := &Store{
		orders: make(map[int]*Order),
		nextID: 1,
		path:   path,
	}
	s.load()
	return s
}

func (s *Store) load() {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return // no file yet - start with an empty board
	}
	var snap snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return
	}
	for _, o := range snap.Orders {
		s.orders[o.ID] = o
	}
	if snap.NextID > 0 {
		s.nextID = snap.NextID
	}
}

// saveLocked writes the current state to disk. Caller must hold s.mu.
func (s *Store) saveLocked() error {
	orders := make([]*Order, 0, len(s.orders))
	for _, o := range s.orders {
		orders = append(orders, o)
	}
	sort.Slice(orders, func(i, j int) bool { return orders[i].ID < orders[j].ID })

	data, err := json.MarshalIndent(snapshot{Orders: orders, NextID: s.nextID}, "", "  ")
	if err != nil {
		return err
	}

	// Write to a temp file then rename, so a crash mid-write can't corrupt orders.json.
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Create adds a new pending order for the given day and persists it.
func (s *Store) Create(items OrderItems, note, source, forDate string) (*Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	o := &Order{
		ID:        s.nextID,
		Items:     items,
		Note:      note,
		Source:    source,
		ForDate:   forDate,
		Status:    "pending",
		CreatedAt: time.Now(),
	}
	s.orders[o.ID] = o
	s.nextID++

	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return o, nil
}

// Update edits a pending order's contents in place (used when the counter
// taps "Edit" on a ticket that hasn't gone out yet).
func (s *Store) Update(id int, items OrderItems, note, source, forDate string) (*Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	o, ok := s.orders[id]
	if !ok {
		return nil, ErrNotFound
	}
	o.Items = items
	o.Note = note
	o.Source = source
	o.ForDate = forDate

	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return o, nil
}

// List returns orders, optionally filtered by status ("pending"/"done") and/or
// forDate ("YYYY-MM-DD"), oldest first. Empty strings mean "no filter".
func (s *Store) List(status, forDate string) []*Order {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]*Order, 0, len(s.orders))
	for _, o := range s.orders {
		if status != "" && o.Status != status {
			continue
		}
		if forDate != "" && o.ForDate != forDate {
			continue
		}
		out = append(out, o)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

// Complete marks an order done.
func (s *Store) Complete(id int) (*Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	o, ok := s.orders[id]
	if !ok {
		return nil, ErrNotFound
	}
	now := time.Now()
	o.Status = "done"
	o.CompletedAt = &now

	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return o, nil
}

// Reopen undoes an accidental "done" tap, putting the order back on the board.
func (s *Store) Reopen(id int) (*Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	o, ok := s.orders[id]
	if !ok {
		return nil, ErrNotFound
	}
	o.Status = "pending"
	o.CompletedAt = nil

	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return o, nil
}

// Delete removes an order entirely (a mistaken entry or a cancellation).
func (s *Store) Delete(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.orders[id]; !ok {
		return ErrNotFound
	}
	delete(s.orders, id)
	return s.saveLocked()
}

// ClearCompleted wipes "done" orders. If forDate is empty, every done order
// is cleared; otherwise only done orders filed for that date are.
func (s *Store) ClearCompleted(forDate string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	n := 0
	for id, o := range s.orders {
		if o.Status != "done" {
			continue
		}
		if forDate != "" && o.ForDate != forDate {
			continue
		}
		delete(s.orders, id)
		n++
	}
	s.saveLocked()
	return n
}

// ItemCount is one row of a per-item breakdown.
type ItemCount struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Qty  int    `json:"qty"`
}

// Summary is the full "Day Summary" view: what's still on the grill, what's
// gone out split by channel, and the money that matches.
type Summary struct {
	Date            string      `json:"date"`
	Grill           []ItemCount `json:"grill"`
	ServedInPerson  []ItemCount `json:"servedInPerson"`
	ServedDelivery  []ItemCount `json:"servedDelivery"`
	OrdersPending   int         `json:"ordersPending"`
	OrdersInPerson  int         `json:"ordersInPerson"`
	OrdersDelivery  int         `json:"ordersDelivery"`
	RevenueInPerson float64     `json:"revenueInPerson"`
	RevenueDelivery float64     `json:"revenueDelivery"`
	RevenueTotal    float64     `json:"revenueTotal"`
}

func (s *Store) Summary(forDate string) Summary {
	s.mu.Lock()
	defer s.mu.Unlock()

	grillQty := map[string]int{}
	inPersonQty := map[string]int{}
	deliveryQty := map[string]int{}
	ordersPending, ordersInPerson, ordersDelivery := 0, 0, 0
	revenueInPerson, revenueDelivery := 0.0, 0.0

	for _, o := range s.orders {
		if o.ForDate != forDate {
			continue
		}
		switch o.Status {
		case "pending":
			ordersPending++
			for id, qty := range o.Items {
				grillQty[id] += qty
			}
		case "done":
			if IsDelivery(o.Source) {
				ordersDelivery++
				revenueDelivery += ItemTotal(o.Items)
				for id, qty := range o.Items {
					deliveryQty[id] += qty
				}
			} else {
				ordersInPerson++
				revenueInPerson += ItemTotal(o.Items)
				for id, qty := range o.Items {
					inPersonQty[id] += qty
				}
			}
		}
	}

	toRows := func(qty map[string]int) []ItemCount {
		rows := make([]ItemCount, 0, len(qty))
		for _, mi := range Menu {
			if q := qty[mi.ID]; q > 0 {
				rows = append(rows, ItemCount{ID: mi.ID, Name: mi.Name, Qty: q})
			}
		}
		return rows
	}

	return Summary{
		Date:            forDate,
		Grill:           toRows(grillQty),
		ServedInPerson:  toRows(inPersonQty),
		ServedDelivery:  toRows(deliveryQty),
		OrdersPending:   ordersPending,
		OrdersInPerson:  ordersInPerson,
		OrdersDelivery:  ordersDelivery,
		RevenueInPerson: revenueInPerson,
		RevenueDelivery: revenueDelivery,
		RevenueTotal:    revenueInPerson + revenueDelivery,
	}
}
