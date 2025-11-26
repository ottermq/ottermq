package api

import (
	"net/url"

	"github.com/andrelcunha/ottermq/internal/core/amqp/errors"
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
	queues := b.Management.ListQueues()

	return c.Status(fiber.StatusOK).JSON(models.QueueListResponse{
		Queues: queues,
	})
}

// GetQueue godoc
// @Summary Get a queue
// @Description Get a queue by name
// @Tags queues
// @Accept json
// @Produce json
// @Param vhost path string false "VHost name" default(/)
// @Param queue path string true "Queue name"
// @Success 200 {object} models.QueueDTO
// @Failure 401 {object} models.UnauthorizedErrorResponse "Missing or invalid JWT token"
// @Failure 500 {object} models.ErrorResponse "Failed to list queues"
// @Router /queues/{vhost}/{queue} [get]
// @Security BearerAuth
func GetQueue(c *fiber.Ctx, b *broker.Broker) error {
	vhost := c.Params("vhost")
	if vhost == "" {
		vhost = "/" // default vhost
	} else {
		decoded, err := url.PathUnescape(vhost)
		if err == nil {
			vhost = decoded
		}
	}
	queueName := c.Params("queue")
	if queueName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: "queue name is required",
		})
	}
	queue, err := b.Management.GetQueue(vhost, queueName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error: "Failed to get queue: " + err.Error(),
		})
	}
	return c.Status(fiber.StatusOK).JSON(*queue)
}

// CreateQueue godoc
// @Summary Create a new queue
// @Description Create a new queue with the specified name
// @Tags queues
// @Accept json
// @Produce json
// @Param vhost path string false "VHost name" default(/)
// @Param queue body models.CreateQueueRequest true "Queue to create"
// @Success 200 {object} models.SuccessResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.UnauthorizedErrorResponse "Missing or invalid JWT token"
// @Failure 404 {object} models.ErrorResponse "VHost not found"
// @Failure 404 {object} models.ErrorResponse "Queue not found in vhost"
// @Failure 500 {object} models.ErrorResponse
// @Router /queues [post]
// @Security BearerAuth
func CreateQueue(c *fiber.Ctx, b *broker.Broker) error {
	vhost := c.Params("vhost")
	if vhost == "" {
		vhost = "/" // default vhost
	} else {
		decoded, err := url.PathUnescape(vhost)
		if err == nil {
			vhost = decoded
		}
	}
	var request models.CreateQueueRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: "invalid request body: " + err.Error(),
		})
	}

	_, err := b.Management.CreateQueue(vhost, request)
	if err != nil {
		// if error is amqp error, verify if it's a 404 (not found) and contains 'no queue' in the text
		if err.(errors.AMQPError).ReplyCode() == 404 {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{
				Error: err.(errors.AMQPError).ReplyText(),
			})
		}
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
// @Param vhost path string false "VHost name" default(/)
// @Param queue path string true "Queue name"
// @Success 204 {object} nil
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.UnauthorizedErrorResponse "Missing or invalid JWT token"
// @Failure 500 {object} models.ErrorResponse
// @Router /queues/{queue} [delete]
// @Security BearerAuth
func DeleteQueue(c *fiber.Ctx, b *broker.Broker) error {
	vhost := c.Params("vhost")
	if vhost == "" {
		vhost = "/" // default vhost
	}
	queueName := c.Params("queue")
	if queueName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: "queue name is required",
		})
	}
	ifUnused := c.Query("ifUnused") == "true"
	ifEmpty := c.Query("ifEmpty") == "true"

	err := b.Management.DeleteQueue(vhost, queueName, ifUnused, ifEmpty)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error: err.Error(),
		})
	}

	return c.Status(fiber.StatusNoContent).Send(nil)
}
