package middleware

import (
	jwtware "github.com/gofiber/contrib/jwt"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/ottermq/ottermq/internal/core/models"
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

// AdminOnly rejects requests whose JWT role claim is not "admin".
// Must be placed after JwtMiddleware in the handler chain.
func AdminOnly(c *fiber.Ctx) error {
	token, ok := c.Locals("user").(*jwt.Token)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(models.UnauthorizedErrorResponse{
			Error: "Missing or invalid JWT token",
		})
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(models.UnauthorizedErrorResponse{
			Error: "Invalid token claims",
		})
	}
	role, _ := claims["role"].(string)
	if role != "admin" {
		return c.Status(fiber.StatusForbidden).JSON(models.ErrorResponse{
			Error: "Admin access required",
		})
	}
	return c.Next()
}
