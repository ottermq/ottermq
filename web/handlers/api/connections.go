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

// CloseConnection godoc
// @Summary Close a connection
// @Description Close a specific connection by its ID
// @Tags connections
// @Accept json
// @Produce json
// @Param name path string true "Connection ID"
// @Param reason query string false "Reason for closing the connection"
// @Success 200 {object} models.SuccessResponse
// @Failure 400 {object} models.ErrorResponse "Connection ID is required"
// @Failure 401 {object} models.UnauthorizedErrorResponse "Missing or invalid JWT token"
// @Failure 500 {object} models.ErrorResponse "Failed to close connection"
// @Router /connections/{name} [delete]
// @Security BearerAuth
func CloseConnection(c *fiber.Ctx, b *broker.Broker) error {
	connectionID := c.Params("name")
	if connectionID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: "Connection ID is required",
		})
	} else {
		decoded, err := url.PathUnescape(connectionID)
		if err == nil {
			connectionID = decoded
		}
	}
	reason := c.Query("reason", "Closed by admin via API")
	err := b.Management.CloseConnection(connectionID, reason)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error: "Failed to close connection: " + err.Error(),
		})
	}
	return c.Status(fiber.StatusOK).JSON(models.SuccessResponse{
		Message: "Connection closed",
	})
}
