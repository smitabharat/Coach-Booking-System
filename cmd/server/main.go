// Command server runs the Coach Booking REST API. Swagger UI is served at /swagger/index.html (spec: /swagger/doc.json).
package main

import (
	"log"
	"os"
	"strings"

	_ "time/tzdata" // embed IANA zones (e.g. America/New_York) on minimal OS images

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/swagger"
	"github.com/nudgebee/booking-api/internal/database"
	"github.com/nudgebee/booking-api/internal/handler"
	"github.com/nudgebee/booking-api/internal/openapi"
	"github.com/nudgebee/booking-api/internal/service"
)

func main() {
	dsn := database.ResolveDSN()
	if os.Getenv("DATABASE_URL") == "" && os.Getenv("BOOKING_DB_DSN") == "" && os.Getenv("PGPASSWORD") == "" {
		log.Printf("database: using default local URL %q (set DATABASE_URL, BOOKING_DB_DSN, or PGPASSWORD if your Postgres user/password differ)", database.DefaultLocalDSN)
	}
	db, err := database.Open(dsn)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "refused") || strings.Contains(msg, "actively refused") {
			log.Fatalf("database: %v\nhint: PostgreSQL is not accepting TCP connections on this host/port. Start the PostgreSQL service (Windows: services.msc → PostgreSQL), or set PGPORT / BOOKING_DB_DSN if your server uses a different port.", err)
		}
		log.Fatalf("database: %v", err)
	}

	svc := service.NewBookingService(db)
	h := handler.New(svc)

	app := fiber.New(fiber.Config{
		AppName: "Coach Booking API",
	})
	app.Use(recover.New())
	app.Use(logger.New())

	app.Get("/swagger/doc.json", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "application/json")
		return c.Send(openapi.Spec)
	})
	app.Get("/swagger/*", swagger.New(swagger.Config{
		URL:         "/swagger/doc.json",
		DeepLinking: true,
	}))

	h.Mount(app)

	addr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}
	log.Printf("listening on %s (swagger: http://localhost%s/swagger/index.html)", addr, addr)
	if err := app.Listen(addr); err != nil {
		log.Fatal(err)
	}
}
