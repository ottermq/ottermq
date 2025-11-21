package api

import (
	"github.com/andrelcunha/ottermq/internal/core/broker"
	"github.com/andrelcunha/ottermq/internal/core/models"
	"github.com/gofiber/fiber/v2"
)

// ListConsumers godoc
// @Summary List all consumers
// @Description Get a list of all consumers
// @Tags consumers
// @Accept json
// @Produce json
// @Success 200 {object} models.ConnectionListResponse
// @Failure 401 {object} models.UnauthorizedErrorResponse "Missing or invalid JWT token"
// @Failure 500 {object} models.ErrorResponse "Failed to list connections"
// @Router /connections [get]
// @Security BearerAuth
func ListConsumers(c *fiber.Ctx, b *broker.Broker) error {
	// get query parameters
	vhost := c.Query("vhost", "/")
	consumers, err := b.Management.ListConsumers(vhost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error: "Failed to list consumers: " + err.Error(),
		})
	}
	return c.Status(fiber.StatusOK).JSON(models.ConsumerListResponse{
		Consumers: consumers,
	})
}

// ListQueueConsumers godoc
// @Summary List consumers for a specific queue
// @Description Get a list of consumers for a specific queue
// @Tags consumers
// @Accept json
// @Produce json
// @Param vhost query string false "VHost name" default(/)
// @Param queueName path string true "Queue Name"
// @Success 200 {object} models.ConsumerListResponse
// @Failure 401 {object} models.UnauthorizedErrorResponse "Missing or invalid JWT token"
// @Failure 500 {object} models.ErrorResponse "Failed to list consumers for queue"
// @Router /queues/{queueName}/consumers [get]
// @Security BearerAuth
func ListQueueConsumers(c *fiber.Ctx, b *broker.Broker) error {
	vhost := c.Query("vhost", "/")
	queueName := c.Params("queueName")

	consumers, err := b.Management.ListQueueConsumers(vhost, queueName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error: "Failed to list consumers for queue: " + err.Error(),
		})
	}
	return c.Status(fiber.StatusOK).JSON(models.ConsumerListResponse{
		Consumers: consumers,
	})
}
