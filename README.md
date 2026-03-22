# Coach Booking API

## Project description

This repository is a **backend service** for scheduling **30-minute sessions** between clients (“users”) and coaches. Coaches define **recurring weekly availability** in their own **IANA timezone**; the API turns that into concrete open slots for a given calendar day, applies **bookings** with transactional checks so double-booking is avoided, and supports **listing** and **soft cancellation** of appointments.

**What it does in practice:** register coaches and users, add availability windows by weekday and local time, query free slots (either as **UTC** for coach-oriented clients or in the **coach’s local offset** for user-facing flows), create bookings that must align with an available slot, and manage a user’s booking history.

**Stack:** [Go](https://go.dev/), [Fiber](https://gofiber.io/) for HTTP, [GORM](https://gorm.io/) with **PostgreSQL** for persistence (including migrations and a partial unique index on active bookings). OpenAPI is **embedded** and served as Swagger UI for exploration and testing.

**Interactive docs:** run the server and open [http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html) (OpenAPI JSON: `/swagger/doc.json`).

---

## Run locally

**Prerequisites:** Go 1.22+, PostgreSQL.

1. Create a database (e.g. `booking`). The app can create it automatically on first connect if missing (connects to `postgres` first).

2. Set a connection string (pick one):

   | Variable | Purpose |
   |----------|---------|
   | `DATABASE_URL` | Full Postgres URL (highest priority) |
   | `BOOKING_DB_DSN` | Same, second priority |
   | `PGPASSWORD` (+ optional `PGUSER`, `PGHOST`, `PGPORT`, `PGDATABASE`) | Built into a URL if unset DSN |

   If none are set, a **development default** is used (see `internal/database/database.go`).

3. Start the server:

   ```bash
   cd "path/to/Nudgebee Assignment"
   go run ./cmd/server
   ```

   Optional: `PORT=3000` to change the listen port (default **8080**).

---

## API reference

Base URL: `http://localhost:8080` (adjust host/port as needed).

- **Input (bodies):** `Content-Type: application/json`
- **Output:** JSON with `Content-Type: application/json`, except **`204 No Content`** (empty body)

Endpoints are grouped like the OpenAPI tags in Swagger UI.

### Error responses (all groups)

On failure:

```json
{ "error": "human-readable message" }
```

Typical status codes: `400` (validation / bad request), `403` (forbidden), `404` (not found), `409` (conflict), `500` (server error).

---

### Coaches

**Register coaches and set weekly availability (local wall time).**

| Method | Endpoint | Input summary | Output summary |
|--------|----------|---------------|----------------|
| `POST` | `/coaches` | Body: `name`, `timezone` | `201` — coach JSON |
| `GET` | `/coaches/:id` | Path: `id` | `200` — coach JSON |
| `POST` | `/coaches/availability` | Body: `coach_id`, `day`, `start_time`, `end_time` | `200` — confirmation JSON |

#### `POST /coaches`

Register a coach. `timezone` must be a valid **IANA** name (e.g. `Asia/Kolkata`, `UTC`).

**Input**

| Kind | Field | Type | Required | Description |
|------|-------|------|----------|-------------|
| Body | `name` | string | yes | Display name |
| Body | `timezone` | string | yes | IANA timezone for availability and `date` on slot queries |

**Output — `201 Created`**

```json
{
  "id": 1,
  "name": "Coach A",
  "timezone": "Asia/Kolkata",
  "CreatedAt": "2026-03-21T12:00:00Z",
  "UpdatedAt": "2026-03-21T12:00:00Z"
}
```

**Errors:** `400` invalid JSON or invalid name/timezone.

---

#### `GET /coaches/:id`

**Input**

| Kind | Field | Type | Required | Description |
|------|-------|------|----------|-------------|
| Path | `id` | integer | yes | Coach primary key |

**Output — `200 OK`**

```json
{
  "id": 1,
  "name": "Coach A",
  "timezone": "Asia/Kolkata",
  "CreatedAt": "2026-03-21T12:00:00Z",
  "UpdatedAt": "2026-03-21T12:00:00Z"
}
```

**Errors:** `400` invalid `id`, `404` coach not found.

---

#### `POST /coaches/availability`

Add one recurring weekly window in the coach’s local **24h** `HH:MM`. Call again for other weekdays or ranges.

**Input**

| Kind | Field | Type | Required | Description |
|------|-------|------|----------|-------------|
| Body | `coach_id` | integer | yes | Coach ID |
| Body | `day` | string | yes | `Monday`, … or `0`–`6` (Sunday=0 … Saturday=6) |
| Body | `start_time` | string | yes | e.g. `"09:00"` |
| Body | `end_time` | string | yes | e.g. `"14:00"` (slots are 30 minutes inside the window) |

**Output — `200 OK`**

```json
{
  "message": "Weekly availability saved successfully.",
  "coach_id": 1,
  "day": "Monday",
  "start_time": "09:00",
  "end_time": "14:00"
}
```

**Errors:** `400` invalid JSON, missing `coach_id`, bad weekday/window, `404` coach not found.

---

### Users

**End-user accounts (required for bookings).**

| Method | Endpoint | Input summary | Output summary |
|--------|----------|---------------|----------------|
| `POST` | `/users` | Body: `name`, `email` | `201` — user JSON |
| `GET` | `/users/:id` | Path: `id` | `200` — user JSON |

#### `POST /users`

**Input**

| Kind | Field | Type | Required | Description |
|------|-------|------|----------|-------------|
| Body | `name` | string | yes | Display name |
| Body | `email` | string | yes | Unique email |

**Output — `201 Created`**

```json
{
  "id": 1,
  "name": "Jane",
  "email": "jane@example.com",
  "CreatedAt": "2026-03-21T12:00:00Z",
  "UpdatedAt": "2026-03-21T12:00:00Z"
}
```

**Errors:** `400` invalid JSON or invalid input, `409` email already registered.

---

#### `GET /users/:id`

**Input**

| Kind | Field | Type | Required | Description |
|------|-------|------|----------|-------------|
| Path | `id` | integer | yes | User primary key |

**Output — `200 OK`**

```json
{
  "id": 1,
  "name": "Jane",
  "email": "jane@example.com",
  "CreatedAt": "2026-03-21T12:00:00Z",
  "UpdatedAt": "2026-03-21T12:00:00Z"
}
```

**Errors:** `400` invalid `id`, `404` user not found.

---

### Slots

**Query open 30-minute appointment starts for a coach + calendar day.**

| Method | Endpoint | Input summary | Output summary |
|--------|----------|---------------|----------------|
| `GET` | `/coaches/:coach_id/slots` | Path: `coach_id`; query: `date` | `200` — JSON array of **UTC** RFC3339 strings (`Z`) |
| `GET` | `/users/slots` | Query: `coach_id`, `date` | `200` — JSON array of RFC3339 strings in **coach IANA timezone** |

Both use the same rules: `date` is **`YYYY-MM-DD` on the coach’s local calendar**; only **unbooked** 30-minute starts are returned.

#### `GET /coaches/:coach_id/slots`

**Input**

| Kind | Field | Type | Required | Description |
|------|-------|------|----------|-------------|
| Path | `coach_id` | integer | yes | Coach primary key |
| Query | `date` | string | yes | Coach-local day; longer ISO date prefix allowed (date part only is used) |

**Output — `200 OK`** (JSON array of strings)

```json
["2024-04-24T04:00:00Z", "2024-04-24T04:30:00Z"]
```

`[]` if no available slots.

**Errors:** `400` bad `coach_id` / `date` or invalid coach timezone, `404` coach not found.

---

#### `GET /users/slots`

Same slot set as above; each instant is formatted in the coach’s timezone (RFC3339 with offset, e.g. `+05:30`).

**Input**

| Kind | Field | Type | Required | Description |
|------|-------|------|----------|-------------|
| Query | `coach_id` | integer | yes | Coach primary key |
| Query | `date` | string | yes | `YYYY-MM-DD` (coach-local calendar day) |

**Output — `200 OK`** (JSON array of strings)

```json
["2024-04-24T09:30:00+05:30", "2024-04-24T10:00:00+05:30"]
```

`[]` if no slots.

**Errors:** same as `GET /coaches/:coach_id/slots`.

---

### Bookings

**Book, list, and cancel appointments.**

| Method | Endpoint | Input summary | Output summary |
|--------|----------|---------------|----------------|
| `POST` | `/users/bookings` | Body: `user_id`, `coach_id`, `datetime` | `201` — booking JSON |
| `GET` | `/users/bookings` | Query: `user_id` | `200` — JSON array of booking objects |
| `DELETE` | `/users/bookings/:id` | Path: `id`; query: `user_id` | `204` — no body |

#### `POST /users/bookings`

`datetime` must be **RFC3339** and match an **available** slot (same instant as from a slots endpoint). Prefer the **exact** string from `GET .../slots`.

**Input**

| Kind | Field | Type | Required | Description |
|------|-------|------|----------|-------------|
| Body | `user_id` | integer | yes | Existing user |
| Body | `coach_id` | integer | yes | Existing coach |
| Body | `datetime` | string | yes | RFC3339, e.g. `2024-04-24T04:00:00Z` |

**Output — `201 Created`**

```json
{
  "id": 1,
  "user_id": 1,
  "coach_id": 1,
  "slot_start": "2024-04-24T04:00:00Z",
  "created_at": "2026-03-21T14:00:00Z",
  "user": { "id": 1, "name": "Jane" },
  "coach": { "id": 1, "name": "Coach A" }
}
```

`cancelled_at` appears only when the booking is cancelled.

**Errors:** `400` bad JSON, missing fields, bad `datetime`, slot not bookable; `404` user or coach not found; `409` slot taken.

---

#### `GET /users/bookings`

Lists **non-cancelled** bookings for the user. Nested `user` / `coach` are `{ id, name }` only.

**Input**

| Kind | Field | Type | Required | Description |
|------|-------|------|----------|-------------|
| Query | `user_id` | integer | yes | User primary key |

**Output — `200 OK`** (JSON array)

```json
[
  {
    "id": 1,
    "user_id": 1,
    "coach_id": 1,
    "slot_start": "2024-04-24T04:00:00Z",
    "created_at": "2026-03-21T14:00:00Z",
    "user": { "id": 1, "name": "Jane" },
    "coach": { "id": 1, "name": "Coach A" }
  }
]
```

`[]` if none.

**Errors:** `400` bad `user_id`, `404` user not found.

---

#### `DELETE /users/bookings/:id`

**Input**

| Kind | Field | Type | Required | Description |
|------|-------|------|----------|-------------|
| Path | `id` | integer | yes | Booking primary key |
| Query | `user_id` | integer | yes | Must own the booking |

**Output — `204 No Content`**

No body.

**Errors:** `400` bad ids, `403` wrong user, `404` not found or already cancelled.

---

## Suggested flow (Swagger / testing)
http://localhost:8080/swagger/index.html
1. `POST /users` → note `id`  
2. `POST /coaches` → note `id`  
3. `POST /coaches/availability` (repeat for each weekday/window)  
4. `GET /users/slots` or `GET /coaches/{id}/slots` with correct `date` (weekday must match availability)  
5. `POST /users/bookings` with `datetime` copied from step 4  
6. `GET /users/bookings?user_id=...`  
7. `DELETE /users/bookings/{booking_id}?user_id=...` to cancel  

---

## Tests

Integration tests expect PostgreSQL. Set **`TEST_DATABASE_URL`** or **`BOOKING_DB_DSN`** to a test database, then:

```bash
go test ./...
```

---

## Project layout (short)

| Path | Role |
|------|------|
| `cmd/server` | HTTP entrypoint, Swagger routes |
| `internal/handler` | Fiber handlers & booking JSON DTOs |
| `internal/service` | Booking rules, transactions, preloads |
| `internal/models` | GORM models |
| `internal/database` | Postgres DSN, migrate, partial unique index |
| `internal/openapi` | Embedded `swagger.json` |
| `internal/timeutil` | Weekday / date / time parsing helpers |
