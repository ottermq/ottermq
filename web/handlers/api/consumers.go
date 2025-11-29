package api

import (
	"net/url"

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
// @Success 200 {object} models.ConsumerListResponse
// @Failure 401 {object} models.UnauthorizedErrorResponse "Missing or invalid JWT token"
// @Failure 500 {object} models.ErrorResponse "Failed to list consumers"
// @Router /consumers [get]
// @Security BearerAuth
func ListConsumers(c *fiber.Ctx, b *broker.Broker) error {
	consumers, err := b.Management.ListConsumers()
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
// @Success 200 {object} models.ConsumerListResponse
// @Failure 401 {object} models.UnauthorizedErrorResponse "Missing or invalid JWT token"
// @Failure 500 {object} models.ErrorResponse "Failed to list consumers for queue"
// @Router /consumers/{vhost} [get]
// @Security BearerAuth
func ListVhostConsumers(c *fiber.Ctx, b *broker.Broker) error {
	vhost := c.Params("vhost")
	if vhost == "" {
		vhost = "/" // default vhost
	} else {
		decoded, err := url.PathUnescape(vhost)
		if err == nil {
			vhost = decoded
		}
	}
	consumers, err := b.Management.ListVhostConsumers(vhost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error: "Failed to list consumers for queue: " + err.Error(),
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
