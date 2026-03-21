# Coach-Booking-System
Design and build a RESTful API for a simple appointment booking system. The platform connects coaches with users who want to book 30-minute appointment slots. This task will assess your ability to design a system, model data, build APIs, and handle core business logic.
## Project description

This repository is a **backend service** for scheduling **30-minute sessions** between clients (“users”) and coaches. Coaches define **recurring weekly availability** in their own **IANA timezone**; the API turns that into concrete open slots for a given calendar day, applies **bookings** with transactional checks so double-booking is avoided, and supports **listing** and **soft cancellation** of appointments.

**What it does in practice:** register coaches and users, add availability windows by weekday and local time, query free slots (either as **UTC** for coach-oriented clients or in the **coach’s local offset** for user-facing flows), create bookings that must align with an available slot, and manage a user’s booking history.

**Stack:** [Go](https://go.dev/), [Fiber](https://gofiber.io/) for HTTP, [GORM](https://gorm.io/) with **PostgreSQL** for persistence (including migrations and a partial unique index on active bookings). OpenAPI is **embedded** and served as Swagger UI for exploration and testing.

**Interactive docs:** run the server and open [http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html) (OpenAPI JSON: `/swagger/doc.json`).

---

## Run locally

**Prerequisites:** Go 1.22+, PostgreSQL.

1. Create a database (e.g. `booking`). The app can create it automatically on first connect if missing (connects to `postgres` first).

2. Start the server:

   ```bash
   cd "path/to/Nudgebee Assignment"
   go run ./cmd/server
   ```

   Optional: `PORT=3000` to change the listen port (default **8080**).

---

## API reference

Base URL: `http://localhost:8080` (adjust host/port as needed).

All JSON bodies use `Content-Type: application/json`.

### Coaches

#### `POST /coaches`

Register a coach. `timezone` must be a valid **IANA** name (e.g. `Asia/Kolkata`, `UTC`).

**Request body**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Display name |
| `timezone` | string | yes | IANA timezone for availability and slot dates |

**Response:** `201 Created`

```json
{
  "id": 1,
  "name": "Coach A",
  "timezone": "Asia/Kolkata",
  "CreatedAt": "...",
  "UpdatedAt": "..."
}
```

*(Internal timestamp field names may appear as `CreatedAt` / `UpdatedAt` in JSON.)*

**Errors:** `400` invalid name/timezone.

---

#### `GET /coaches/:id`

Fetch one coach by ID (`id` path parameter).

**Response:** `200 OK` — same shape as create (`id`, `name`, `timezone`, …).

**Errors:** `400` invalid id, `404` not found.

---

#### `POST /coaches/availability`

Add one weekly window (coach-local **24h** `HH:MM`). Call multiple times for more weekdays or ranges.

**Request body**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `coach_id` | number | yes | Coach ID |
| `day` | string | yes | Weekday: name (`Monday`, `Tuesday`, …) or `0`–`6` (Sunday=0 … Saturday=6) |
| `start_time` | string | yes | e.g. `"09:00"` |
| `end_time` | string | yes | e.g. `"14:00"` (exclusive end for last slot edge; slots are 30 minutes) |

**Response:** `200 OK`

```json
{
  "message": "Weekly availability saved successfully.",
  "coach_id": 1,
  "day": "Monday",
  "start_time": "09:00",
  "end_time": "14:00"
}
```

**Errors:** `400` bad JSON / invalid window, `404` coach not found.

---

#### `GET /coaches/:coach_id/slots`

List **available** (not yet booked) 30-minute slot **start** times for one calendar day.

**Query parameters**

| Parameter | Required | Description |
|-----------|----------|-------------|
| `date` | yes | `YYYY-MM-DD` in the **coach’s** timezone (calendar day). A full ISO date-time prefix is accepted; only the date part is used. |

**Response:** `200 OK` — JSON array of strings, each **UTC RFC3339** (suffix `Z`).

```json
["2024-04-24T04:00:00Z", "2024-04-24T04:30:00Z"]
```

**Errors:** `400` missing/invalid `date` or coach timezone, `404` coach not found.

---

### Users

#### `POST /users`

Register a user (needed before booking).

**Request body**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Display name |
| `email` | string | yes | Valid email address; must be unique |

**Response:** `201 Created`

```json
{
  "id": 1,
  "name": "Jane",
  "email": "jane@example.com",
  "CreatedAt": "...",
  "UpdatedAt": "..."
}
```

**Errors:** `400` invalid input, `409` email already registered.

---

#### `GET /users/:id`

Fetch one user by ID.

**Response:** `200 OK` — same fields as create.

**Errors:** `400` invalid id, `404` not found.

---

#### `GET /users/slots`

Same slot logic as `GET /coaches/:coach_id/slots`, but `coach_id` is a **query** parameter and each time string is formatted in the **coach’s IANA timezone** (RFC3339 with offset, e.g. `+05:30`).

**Query parameters**

| Parameter | Required | Description |
|-----------|----------|-------------|
| `coach_id` | yes | Coach ID |
| `date` | yes | `YYYY-MM-DD` (coach-local day) |

**Response:** `200 OK`

```json
["2024-04-24T09:30:00+05:30", "2024-04-24T10:00:00+05:30"]
```

**Errors:** same as coach slots endpoint.

---

### Bookings

#### `POST /users/bookings`

Book one slot. `user_id` and `coach_id` must exist. `datetime` must be **RFC3339** and match an **available** slot (same instant as returned by a slots endpoint). Use the **exact** string from `GET .../slots` when possible.

**Request body**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `user_id` | number | yes | Existing user |
| `coach_id` | number | yes | Existing coach |
| `datetime` | string | yes | RFC3339, e.g. `2024-04-24T04:00:00Z` or `2024-04-24T09:30:00+05:30` |

**Response:** `201 Created`

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

`cancelled_at` appears only if applicable.

**Errors:** `400` bad input / slot not available / not on 30-minute boundary, `404` user or coach not found, `409` slot already taken.

---

#### `GET /users/bookings`

List all **non-cancelled** bookings for a user (past and future).

**Query parameters**

| Parameter | Required | Description |
|-----------|----------|-------------|
| `user_id` | yes | User ID |

**Response:** `200 OK` — array of the same booking object shape as `POST` (with `user` / `coach` summaries: `id`, `name` only).

```json
[
  {
    "id": 1,
    "user_id": 1,
    "coach_id": 1,
    "slot_start": "2024-04-24T04:00:00Z",
    "created_at": "...",
    "user": { "id": 1, "name": "Jane" },
    "coach": { "id": 1, "name": "Coach A" }
  }
]
```

**Errors:** `400` missing `user_id`, `404` user not found.

---

#### `DELETE /users/bookings/:id`

Cancel a booking. Path `id` is the **booking** id.

**Query parameters**

| Parameter | Required | Description |
|-----------|----------|-------------|
| `user_id` | yes | Must own the booking |

**Response:** `204 No Content`

**Errors:** `400` bad ids, `403` wrong user, `404` booking not found / already cancelled.

---

## Suggested flow (Swagger / testing)

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
