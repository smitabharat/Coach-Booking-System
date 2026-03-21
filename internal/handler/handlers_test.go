package handler_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/nudgebee/booking-api/internal/handler"
	"github.com/nudgebee/booking-api/internal/service"
	"github.com/nudgebee/booking-api/internal/testdb"
	"github.com/stretchr/testify/require"
)

func newTestApp(t *testing.T) *fiber.App {
	t.Helper()
	db := testdb.RequireDB(t)
	h := handler.New(service.NewBookingService(db))
	app := fiber.New()
	h.Mount(app)
	return app
}

func TestAPIFlow(t *testing.T) {
	app := newTestApp(t)

	reqU := httptest.NewRequest("POST", "/users", strings.NewReader(`{"name":"Eve","email":"eve@test.com"}`))
	reqU.Header.Set("Content-Type", "application/json")
	resU, err := app.Test(reqU)
	require.NoError(t, err)
	require.Equal(t, 201, resU.StatusCode)
	bodyU, _ := io.ReadAll(resU.Body)
	var user struct {
		ID uint `json:"id"`
	}
	require.NoError(t, json.Unmarshal(bodyU, &user))

	req := httptest.NewRequest("POST", "/coaches", strings.NewReader(`{"name":"CoachEve","timezone":"UTC"}`))
	req.Header.Set("Content-Type", "application/json")
	res, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 201, res.StatusCode)
	body, _ := io.ReadAll(res.Body)
	var coach struct {
		ID uint `json:"id"`
	}
	require.NoError(t, json.Unmarshal(body, &coach))

	req2 := httptest.NewRequest("POST", "/coaches/availability", strings.NewReader(
		fmt.Sprintf(`{"coach_id":%d,"day":"Monday","start_time":"10:00","end_time":"11:00"}`, coach.ID),
	))
	req2.Header.Set("Content-Type", "application/json")
	res2, err := app.Test(req2)
	require.NoError(t, err)
	require.Equal(t, 200, res2.StatusCode)

	req3 := httptest.NewRequest("GET", fmt.Sprintf("/users/slots?coach_id=%d&date=2024-01-01", coach.ID), nil)
	res3, err := app.Test(req3)
	require.NoError(t, err)
	require.Equal(t, 200, res3.StatusCode)
	var slots []string
	require.NoError(t, json.NewDecoder(res3.Body).Decode(&slots))
	require.Len(t, slots, 2)

	reqCoachSlots := httptest.NewRequest("GET", fmt.Sprintf("/coaches/%d/slots?date=2024-01-01", coach.ID), nil)
	resCoachSlots, err := app.Test(reqCoachSlots)
	require.NoError(t, err)
	require.Equal(t, 200, resCoachSlots.StatusCode)
	var slotsViaCoach []string
	require.NoError(t, json.NewDecoder(resCoachSlots.Body).Decode(&slotsViaCoach))
	require.Equal(t, slots, slotsViaCoach)

	bookBody := fmt.Sprintf(`{"user_id":%d,"coach_id":%d,"datetime":%q}`, user.ID, coach.ID, slots[0])
	req4 := httptest.NewRequest("POST", "/users/bookings", strings.NewReader(bookBody))
	req4.Header.Set("Content-Type", "application/json")
	res4, err := app.Test(req4)
	require.NoError(t, err)
	require.Equal(t, 201, res4.StatusCode)

	req5 := httptest.NewRequest("GET", fmt.Sprintf("/users/bookings?user_id=%d", user.ID), nil)
	res5, err := app.Test(req5)
	require.NoError(t, err)
	require.Equal(t, 200, res5.StatusCode)
}
