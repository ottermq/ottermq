package api

import (
	"github.com/andrelcunha/ottermq/internal/core/broker"
	"github.com/andrelcunha/ottermq/internal/core/models"
	"github.com/gofiber/fiber/v2"
)

// ListConnections godoc
// @Summary List all connections
// @Description Get a list of all connections
// @Tags connections
// @Accept json
// @Produce json
// @Success 200 {object} models.ConnectionListResponse
// @Failure 401 {object} models.UnauthorizedErrorResponse "Missing or invalid JWT token"
// @Failure 500 {object} models.ErrorResponse "Failed to list connections"
// @Router /connections [get]
// @Security BearerAuth
func ListConnections(c *fiber.Ctx, b *broker.Broker) error {
	connections, err := b.Management.ListConnections()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error: "Failed to list connections: " + err.Error(),
		})
	}
	return c.Status(fiber.StatusOK).JSON(models.ConnectionListResponse{
		Connections: connections,
	})
}
