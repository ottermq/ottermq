package api

import (
	"github.com/andrelcunha/ottermq/internal/core/broker"
	"github.com/andrelcunha/ottermq/internal/core/models"

	"github.com/gofiber/fiber/v2"
)

// ListQueues godoc
// @Summary List all queues
// @Description Get a list of all queues
// @Tags queues
// @Accept json
// @Produce json
// @Success 200 {object} models.QueueListResponse
// @Failure 401 {object} models.UnauthorizedErrorResponse "Missing or invalid JWT token"
// @Failure 500 {object} models.ErrorResponse "Failed to list queues"
// @Router /queues [get]
// @Security BearerAuth
func ListQueues(c *fiber.Ctx, b *broker.Broker) error {
	queues, err := b.Management.ListQueues("/") // Use default vhost for now TODO: allow specifying vhost in the request
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error: "Failed to list queues: " + err.Error(),
		})
	}
	return c.Status(fiber.StatusOK).JSON(models.QueueListResponse{
		Queues: queues,
	})
}

// CreateQueue godoc
// @Summary Create a new queue
// @Description Create a new queue with the specified name
// @Tags queues
// @Accept json
// @Produce json
// @Param queue body models.CreateQueueRequest true "Queue to create"
// @Success 200 {object} models.SuccessResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.UnauthorizedErrorResponse "Missing or invalid JWT token"
// @Failure 500 {object} models.ErrorResponse
// @Router /queues [post]
// @Security BearerAuth
func CreateQueue(c *fiber.Ctx, b *broker.Broker) error {
	var request models.CreateQueueRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: "invalid request body: " + err.Error(),
		})
	}
	if request.QueueName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: "queue name is required",
		})
	}

	_, err := b.Management.CreateQueue(request)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error: err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(models.SuccessResponse{
		Message: "Queue created successfully",
	})
}

// DeleteQueue godoc
// @Summary Delete a queue
// @Description Delete a queue with the specified name
// @Tags queues
// @Accept json
// @Produce json
// @Param queue path string true "Queue name"
// @Success 204 {object} nil
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.UnauthorizedErrorResponse "Missing or invalid JWT token"
// @Failure 500 {object} models.ErrorResponse
// @Router /queues/{queue} [delete]
// @Security BearerAuth
func DeleteQueue(c *fiber.Ctx, b *broker.Broker) error {
	queueName := c.Params("queue")
	if queueName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: "queue name is required",
		})
	}

	// Use default vhost TODO: allow specifying vhost in the request
	// TODO: support ifUnused and ifEmpty query parameters
	err := b.Management.DeleteQueue("/", queueName, false, false)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error: err.Error(),
		})
	}

	return c.Status(fiber.StatusNoContent).Send(nil)
}

// GetMessage godoc
// @Summary Consume a message from a queue
// @Description Consume a message from the specified queue
// @Tags queues
// @Accept json
// @Produce json
// @Param queue path string true "Queue name"
// @Success 200 {object} models.SuccessResponse
// @Failure 400 {object} models.ErrorResponse "Queue name is required"
// @Failure 401 {object} models.UnauthorizedErrorResponse "Missing or invalid JWT token"
// @Failure 404 {object} models.ErrorResponse "No messages in queue"
// @Failure 500 {object} models.ErrorResponse
// @Router /queues/{queue}/consume [post]
// @Security BearerAuth
func GetMessage(c *fiber.Ctx, b *broker.Broker) error {
	queueName := c.Params("queue")
	if queueName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: "Queue name is required",
		})
	}
	msgs, err := b.Management.GetMessages("/", queueName, 1, "ack")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error: err.Error(),
		})
	}
	if len(msgs) == 0 {
		return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{
			Error: "No messages in queue",
		})
	}
	msg := msgs[0]
	return c.Status(fiber.StatusOK).JSON(models.SuccessResponse{
		Message: string(msg.Payload),
	})
}
