package httpapi

import (
	"github.com/gofiber/fiber/v2"
)

type envelope struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Count   int    `json:"count,omitempty"`
	Error   string `json:"error,omitempty"`
}

func sendJSON(c *fiber.Ctx, status int, v any) error {
	return c.Status(status).JSON(v)
}

func okResp(c *fiber.Ctx, data any) error {
	return c.Status(fiber.StatusOK).JSON(envelope{Success: true, Data: data})
}

func okCountResp(c *fiber.Ctx, data any, count int) error {
	return c.Status(fiber.StatusOK).JSON(envelope{Success: true, Data: data, Count: count})
}

func failResp(c *fiber.Ctx, status int, msg string) error {
	return c.Status(status).JSON(envelope{Success: false, Error: msg})
}
