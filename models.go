package main

import "time"

// OrderItems maps a menu item id -> quantity ordered.
type OrderItems map[string]int

// Order is a single ticket, filed under the day it's FOR (not just taken).
type Order struct {
	ID          int        `json:"id"`
	Items       OrderItems `json:"items"`
	Note        string     `json:"note"`
	Source      string     `json:"source"`  // "call", "message", "walkin", "justeat", "ubereats", or ""
	ForDate     string     `json:"forDate"` // "YYYY-MM-DD" - the day this order is FOR
	Status      string     `json:"status"`  // "pending" or "done"
	CreatedAt   time.Time  `json:"createdAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
}

// MenuItem is one line on Chizy Shawarma's board.
type MenuItem struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Category string  `json:"category"` // wraps | sides | meals | drinks | addons
}

// Menu is the full board, in display order.
var Menu = []MenuItem{
	{"chicken", "Shawarma Chicken", 7.00, "wraps"},
	{"beef", "Shawarma Beef", 8.00, "wraps"},
	{"combo", "Shawarma Combo", 9.00, "wraps"},
	{"hotdog", "Classic Hot Dog", 2.50, "sides"},
	{"fries", "Fries", 2.50, "sides"},
	{"pie", "Meat Pie", 2.00, "sides"},
	{"extrasauce", "Extra Sauce", 1.50, "sides"},
	{"extrameat", "Extra Meat", 2.00, "sides"},
	{"shawarmameal", "Shawarma Meal", 10.00, "meals"},
	{"chixchips", "Chicken & Chips", 5.00, "meals"},
	{"wingschips", "Chicken Wings & Chips", 8.50, "meals"},
	{"rio", "Rio", 1.50, "drinks"},
	{"pepsi", "Pepsi", 1.50, "drinks"},
	{"water", "Water", 1.00, "drinks"},
	{"extraveg", "Extra Veggies", 2.00, "addons"},
	{"bananaslice", "Banana Bread Slice", 2.50, "addons"},
	{"bananaloaf", "Banana Bread Loaf", 12.00, "addons"},
}

// MenuByID indexes the menu for quick lookups.
var MenuByID = buildMenuIndex()

func buildMenuIndex() map[string]MenuItem {
	m := make(map[string]MenuItem, len(Menu))
	for _, it := range Menu {
		m[it.ID] = it
	}
	return m
}

// DeliverySources are the channels that count as "Delivery" rather than
// "In-person" in the summary breakdown.
var DeliverySources = map[string]bool{"justeat": true, "ubereats": true}

func IsDelivery(source string) bool {
	return DeliverySources[source]
}

// ItemTotal prices out an item map using the menu.
func ItemTotal(items OrderItems) float64 {
	total := 0.0
	for id, qty := range items {
		if mi, ok := MenuByID[id]; ok {
			total += float64(qty) * mi.Price
		}
	}
	return total
}

const dateLayout = "2006-01-02"

func todayString() string {
	return time.Now().Format(dateLayout)
}

func isValidDate(s string) bool {
	_, err := time.Parse(dateLayout, s)
	return err == nil
}
