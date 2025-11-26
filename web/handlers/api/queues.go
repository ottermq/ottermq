package api

import (
	"fmt"
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
// @Param queue path string true "Queue name -- if empty, a random name will be generated"
// @Param request body models.CreateQueueRequest true "Queue to create"
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
	queue := c.Params("queue")
	if queue != "" {
		decoded, err := url.PathUnescape(queue)
		if err == nil {
			queue = decoded
		}
	}
	var request models.CreateQueueRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: "invalid request body: " + err.Error(),
		})
	}

	q, err := b.Management.CreateQueue(vhost, queue, request)
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
		Message: fmt.Sprintf("Queue %s created successfully", q.Name),
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
// @Param ifUnused query string false "If true, the server will only delete the queue if it has no consumers"
// @Param ifEmpty query string false "If true, the server will only delete the queue if it is empty"
// @Success 204 {object} nil
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.UnauthorizedErrorResponse "Missing or invalid JWT token"
// @Failure 500 {object} models.ErrorResponse
// @Router /queues/{vhost}/{queue} [delete]
// @Security BearerAuth
func DeleteQueue(c *fiber.Ctx, b *broker.Broker) error {
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

// PurgeQueue godoc
// @Summary Purge a queue
// @Description Remove all messages from a queue with the specified name
// @Tags queues
// @Accept json
// @Produce json
// @Param vhost path string false "VHost name" default(/)
// @Param queue path string true "Queue name"
// @Success 204 {object} string "Number of messages purged"
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.UnauthorizedErrorResponse "Missing or invalid JWT token"
// @Failure 500 {object} models.ErrorResponse
// @Router /queues/{vhost}/{queue}/content [delete]
// @Security BearerAuth
func PurgeQueue(c *fiber.Ctx, b *broker.Broker) error {
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

	count, err := b.Management.PurgeQueue(vhost, queueName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error: err.Error(),
		})
	}

	return c.Status(fiber.StatusNoContent).Send([]byte(fmt.Sprintf("Number of messages purged: %d", count)))
}

// ListQueueBindings godoc
// @Summary List all bindings for a queue
// @Description Get a list of all bindings for the specified queue
// @Tags queues
// @Accept json
// @Produce json
// @Param queue path string true "Queue name"
// @Param vhost path string false "VHost name" default(/)
// @Success 200 {object} models.BindingListResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.UnauthorizedErrorResponse "Missing or invalid JWT token"
// @Failure 500 {object} models.ErrorResponse
// @Router /queues/{vhost}/{queue}/bindings [get]
// @Security BearerAuth
func ListQueueBindings(c *fiber.Ctx, b *broker.Broker) error {
	encodedQueueName := c.Params("queue")
	queueName, err := url.PathUnescape(encodedQueueName)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: "Invalid queue name",
		})
	}
	if queueName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: "Queue name is required",
		})
	}
	vhost := c.Params("vhost")
	if vhost == "" {
		vhost = "/" // default vhost
	} else {
		decoded, err := url.PathUnescape(vhost)
		if err == nil {
			vhost = decoded
		}
	}

	bindings, err := b.Management.ListQueueBindings(vhost, queueName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error: "failed to list bindings: " + err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(models.BindingListResponse{
		Bindings: bindings,
	})
}
