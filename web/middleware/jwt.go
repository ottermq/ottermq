package middleware

import (
	"github.com/ottermq/ottermq/internal/core/models"
	jwtware "github.com/gofiber/contrib/jwt"
	"github.com/gofiber/fiber/v2"
)

func JwtMiddleware(secret string) fiber.Handler {
	return jwtware.New(jwtware.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return c.Status(fiber.StatusUnauthorized).JSON(models.UnauthorizedErrorResponse{
				Error: "Missing or invalid JWT token",
			})
		},
		SigningKey: jwtware.SigningKey{Key: []byte(secret)},
	})
}
