# Chizy Shawarma — Order Board Backend

A dependency-free Go API matching the current order board: the full 17-item
menu with prices, date-scoped orders, five order sources, and an
in-person/delivery breakdown. Orders live in memory for speed and are
mirrored to a JSON file (`orders.json` by default) on every change, so a
restart doesn't lose anything.

Just the Go standard library — no third-party packages, so it builds
anywhere Go 1.22+ is installed.

## Run it

```bash
cd shawarma-backend
go run .
```

Starts on `:8080`. Override with environment variables:

```bash
ADDR=:9090 DATA_FILE=/var/lib/shawarma/orders.json go run .
```

Build a single binary:

```bash
go build -o shawarma-server .
./shawarma-server
```

## Menu

17 items across 5 categories — matches the board exactly:

| id | name | price | category |
|---|---|---|---|
| chicken | Shawarma Chicken | £7.00 | wraps |
| beef | Shawarma Beef | £8.00 | wraps |
| combo | Shawarma Combo | £9.00 | wraps |
| hotdog | Classic Hot Dog | £2.50 | sides |
| fries | Fries | £2.50 | sides |
| pie | Meat Pie | £2.00 | sides |
| extrasauce | Extra Sauce | £1.50 | sides |
| extrameat | Extra Meat | £2.00 | sides |
| shawarmameal | Shawarma Meal | £10.00 | meals |
| chixchips | Chicken & Chips | £5.00 | meals |
| wingschips | Chicken Wings & Chips | £8.50 | meals |
| rio | Rio | £1.50 | drinks |
| pepsi | Pepsi | £1.50 | drinks |
| water | Water | £1.00 | drinks |
| extraveg | Extra Veggies | £2.00 | addons |
| bananaslice | Banana Bread Slice | £2.50 | addons |
| bananaloaf | Banana Bread Loaf | £12.00 | addons |

Order sources: `call`, `message`, `walkin`, `justeat`, `ubereats`.
`justeat` and `ubereats` count as **Delivery**; everything else (including
no source at all) counts as **In-person**.

## API

| Method | Path | Description |
|--------|------|--------------|
| GET | `/api/menu` | Full menu with prices |
| POST | `/api/orders` | Create an order |
| GET | `/api/orders` | List orders — `?status=pending\|done` and/or `?date=YYYY-MM-DD` |
| PUT | `/api/orders/{id}` | Edit a pending order's items/note/source/date |
| POST | `/api/orders/{id}/complete` | Mark an order done |
| POST | `/api/orders/{id}/reopen` | Undo an accidental "done" |
| DELETE | `/api/orders/{id}` | Cancel/remove an order |
| POST | `/api/completed/clear` | Clear done orders — `?date=YYYY-MM-DD` (omit to clear all) |
| GET | `/api/summary` | Day summary — `?date=YYYY-MM-DD` (defaults to today) |
| GET | `/api/health` | Liveness check |

### Create an order

```bash
curl -X POST localhost:8080/api/orders \
  -H 'Content-Type: application/json' \
  -d '{
        "items": {"chicken": 2, "combo": 1},
        "note": "Musa - extra spicy",
        "source": "call",
        "forDate": "2026-08-07"
      }'
```

`forDate` is optional — it defaults to today's date on the server if
omitted. Use a future date for phone pre-orders.

### Edit a pending order

```bash
curl -X PUT localhost:8080/api/orders/1 \
  -H 'Content-Type: application/json' \
  -d '{
        "items": {"chicken": 3, "fries": 1},
        "note": "Musa - extra spicy, no onion",
        "source": "call",
        "forDate": "2026-08-07"
      }'
```

### Day summary

```bash
curl "localhost:8080/api/summary?date=2026-08-07"
```

```json
{
  "date": "2026-08-07",
  "grill": [{ "id": "chicken", "name": "Shawarma Chicken", "qty": 2 }],
  "servedInPerson": [{ "id": "combo", "name": "Shawarma Combo", "qty": 1 }],
  "servedDelivery": [],
  "ordersPending": 1,
  "ordersInPerson": 1,
  "ordersDelivery": 0,
  "revenueInPerson": 9.0,
  "revenueDelivery": 0.0,
  "revenueTotal": 9.0
}
```

This maps directly onto the "On the Grill" tally and the "Served —
In-person / Delivery" cards in the app.

## Notes

- **Concurrency-safe**: every write goes through a mutex, fine for a phone,
  a counter tablet, and a walk-in ticket all hitting the API at once.
- **CORS is wide open** (`Access-Control-Allow-Origin: *`) for local,
  single-shop use. Lock it down if this is ever exposed to the internet.
- **No auth**, same reasoning — put it behind a reverse proxy or add a
  shared token if it becomes reachable outside the shop's network.
- To point the React board at this instead of `localStorage`, swap the
  local persistence calls for `fetch` calls to these endpoints — happy to
  do that wiring next if useful.
