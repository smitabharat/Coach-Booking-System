package handler

import (
	"context"
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/nudgebee/booking-api/internal/models"
	"github.com/nudgebee/booking-api/internal/service"
	"github.com/nudgebee/booking-api/internal/timeutil"
)

type Handler struct {
	svc *service.BookingService
}

func New(svc *service.BookingService) *Handler {
	return &Handler{svc: svc}
}

// Mount registers REST routes on the Fiber app.
// Static paths under /users must be registered before /users/:id so "slots" and "bookings" are not captured as ids.
func (h *Handler) Mount(app *fiber.App) {
	app.Post("/coaches", h.CreateCoach)
	app.Post("/coaches/availability", h.SetAvailability)
	app.Get("/coaches/:coach_id/slots", h.ListSlotsForCoach)
	app.Get("/coaches/:id", h.GetCoach)

	app.Get("/users/slots", h.ListSlots)
	app.Post("/users/bookings", h.Book)
	app.Get("/users/bookings", h.ListBookings)
	app.Delete("/users/bookings/:id", h.CancelBooking)
	app.Post("/users", h.CreateUser)
	app.Get("/users/:id", h.GetUser)
}

type createCoachReq struct {
	Name     string `json:"name"`
	Timezone string `json:"timezone"`
}

func (h *Handler) CreateCoach(c *fiber.Ctx) error {
	var req createCoachReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON")
	}
	coach, err := h.svc.CreateCoach(req.Name, req.Timezone)
	if err != nil {
		return mapServiceErr(err)
	}
	return c.Status(fiber.StatusCreated).JSON(coach)
}

func (h *Handler) GetCoach(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return fiber.NewError(fiber.StatusBadRequest, "invalid coach id")
	}
	coach, err := h.svc.GetCoach(uint(id))
	if err != nil {
		return mapServiceErr(err)
	}
	return c.JSON(coach)
}

type createUserReq struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (h *Handler) CreateUser(c *fiber.Ctx) error {
	var req createUserReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON")
	}
	u, err := h.svc.CreateUser(req.Name, req.Email)
	if err != nil {
		return mapServiceErr(err)
	}
	return c.Status(fiber.StatusCreated).JSON(u)
}

func (h *Handler) GetUser(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return fiber.NewError(fiber.StatusBadRequest, "invalid user id")
	}
	u, err := h.svc.GetUser(uint(id))
	if err != nil {
		return mapServiceErr(err)
	}
	return c.JSON(u)
}

type setAvailabilityReq struct {
	CoachID   uint   `json:"coach_id"`
	Day       string `json:"day"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

type setAvailabilityResp struct {
	Message   string `json:"message"`
	CoachID   uint   `json:"coach_id"`
	Day       string `json:"day"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

func (h *Handler) SetAvailability(c *fiber.Ctx) error {
	var req setAvailabilityReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON")
	}
	if req.CoachID == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "coach_id required")
	}
	wd, err := timeutil.ParseWeekday(req.Day)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	if err := h.svc.SetAvailability(req.CoachID, wd, req.StartTime, req.EndTime); err != nil {
		return mapServiceErr(err)
	}
	return c.Status(fiber.StatusOK).JSON(setAvailabilityResp{
		Message:   "Weekly availability saved successfully.",
		CoachID:   req.CoachID,
		Day:       wd.String(),
		StartTime: strings.TrimSpace(req.StartTime),
		EndTime:   strings.TrimSpace(req.EndTime),
	})
}

// ListSlots is GET /users/slots?coach_id=&date=
// date is the coach-local calendar day; each slot is RFC3339 in the coach's IANA timezone.
func (h *Handler) ListSlots(c *fiber.Ctx) error {
	return h.writeAvailableSlots(c, c.Query("coach_id"), c.Query("date"), false)
}

// ListSlotsForCoach is GET /coaches/:coach_id/slots?date= — slot times are UTC RFC3339 (Z), no timezone conversion in the response.
func (h *Handler) ListSlotsForCoach(c *fiber.Ctx) error {
	return h.writeAvailableSlots(c, c.Params("coach_id"), c.Query("date"), true)
}

// utcResponse: true → format each slot as UTC (for /coaches/.../slots); false → format in coach's IANA timezone (for /users/slots).
func (h *Handler) writeAvailableSlots(c *fiber.Ctx, coachIDRaw, date string, utcResponse bool) error {
	if coachIDRaw == "" {
		return fiber.NewError(fiber.StatusBadRequest, "coach_id required")
	}
	cid, err := strconv.ParseUint(coachIDRaw, 10, 64)
	if err != nil || cid == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "coach_id required")
	}
	date = strings.TrimSpace(date)
	if date == "" {
		return fiber.NewError(fiber.StatusBadRequest, "date required (YYYY-MM-DD, coach-local calendar day)")
	}
	slots, err := h.svc.AvailableSlotsUTC(uint(cid), date)
	if err != nil {
		return mapServiceErr(err)
	}
	out := make([]string, len(slots))
	if utcResponse {
		for i, t := range slots {
			out[i] = t.UTC().Format(time.RFC3339)
		}
		return c.JSON(out)
	}
	coach, err := h.svc.GetCoach(uint(cid))
	if err != nil {
		return mapServiceErr(err)
	}
	loc, err := time.LoadLocation(coach.Timezone)
	if err != nil {
		return mapServiceErr(service.ErrInvalidTimezone)
	}
	for i, t := range slots {
		out[i] = t.In(loc).Format(time.RFC3339)
	}
	return c.JSON(out)
}

type bookReq struct {
	UserID   uint   `json:"user_id"`
	CoachID  uint   `json:"coach_id"`
	DateTime string `json:"datetime"`
}

func (h *Handler) Book(c *fiber.Ctx) error {
	var req bookReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON")
	}
	if req.UserID == 0 || req.CoachID == 0 || req.DateTime == "" {
		return fiber.NewError(fiber.StatusBadRequest, "user_id, coach_id, datetime required")
	}
	ts, err := time.Parse(time.RFC3339, req.DateTime)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "datetime must be RFC3339")
	}
	b, err := h.svc.BookSlot(req.UserID, req.CoachID, ts)
	if err != nil {
		return mapServiceErr(err)
	}
	return c.Status(fiber.StatusCreated).JSON(toBookingOut(b))
}

func (h *Handler) ListBookings(c *fiber.Ctx) error {
	uid, err := strconv.ParseUint(c.Query("user_id"), 10, 64)
	if err != nil || uid == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "user_id required")
	}
	list, err := h.svc.ListUserBookings(uint(uid))
	if err != nil {
		return mapServiceErr(err)
	}
	if list == nil {
		list = []models.Booking{}
	}
	return c.JSON(toBookingOutList(list))
}

func (h *Handler) CancelBooking(c *fiber.Ctx) error {
	bid, err := c.ParamsInt("id")
	if err != nil || bid <= 0 {
		return fiber.NewError(fiber.StatusBadRequest, "invalid booking id")
	}
	uid, err := strconv.ParseUint(c.Query("user_id"), 10, 64)
	if err != nil || uid == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "user_id required")
	}
	if err := h.svc.CancelBooking(uint(uid), uint(bid)); err != nil {
		return mapServiceErr(err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func mapServiceErr(err error) error {
	switch {
	case errors.Is(err, service.ErrCoachNotFound), errors.Is(err, service.ErrUserNotFound),
		errors.Is(err, service.ErrBookingNotFound):
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrInvalidSlot), errors.Is(err, service.ErrInvalidDate),
		errors.Is(err, service.ErrInvalidDayOrWindow), errors.Is(err, service.ErrInvalidCoachInput),
		errors.Is(err, service.ErrInvalidUserInput):
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrBookingConflict), errors.Is(err, service.ErrDuplicateEmail):
		return fiber.NewError(fiber.StatusConflict, err.Error())
	case errors.Is(err, service.ErrForbidden):
		return fiber.NewError(fiber.StatusForbidden, err.Error())
	case errors.Is(err, service.ErrInvalidTimezone):
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	default:
		return mapUnexpectedInternalErr(err)
	}
}

func mapUnexpectedInternalErr(err error) error {
	// Log complete error for server-side debugging/root-cause analysis.
	log.Printf("unexpected internal error: %v", err)

	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return fiber.NewError(fiber.StatusGatewayTimeout, "request timed out")
	case errors.Is(err, context.Canceled):
		return fiber.NewError(fiber.StatusRequestTimeout, "request canceled")
	}

	msg := strings.ToLower(err.Error())
	switch {
	// DB/network availability classes. Keep response safe but actionable.
	case strings.Contains(msg, "connection refused"),
		strings.Contains(msg, "dial tcp"),
		strings.Contains(msg, "broken pipe"),
		strings.Contains(msg, "connection reset"),
		strings.Contains(msg, "sqlstate 57p01"),
		strings.Contains(msg, "database system is shutting down"):
		return fiber.NewError(fiber.StatusServiceUnavailable, "database unavailable")
	case strings.Contains(msg, "timeout"),
		strings.Contains(msg, "i/o timeout"):
		return fiber.NewError(fiber.StatusGatewayTimeout, "database timeout")
	default:
		return fiber.NewError(fiber.StatusInternalServerError, "internal server error")
	}
}
