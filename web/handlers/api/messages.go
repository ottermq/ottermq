package api

import (
	"net/url"

	"github.com/andrelcunha/ottermq/internal/core/broker"
	"github.com/andrelcunha/ottermq/internal/core/models"
	"github.com/gofiber/fiber/v2"
)

// PublishMessage godoc
// @Summary Publish a message to an exchange
// @Description Publish a message to the specified exchange with a routing key
// @Tags messages
// @Accept json
// @Produce json
// @Param vhost path string false "VHost name" default(/)
// @Param exchange path string true "Exchange name"
// @Param message body models.PublishMessageRequest true "Message details"
// @Success 200 {object} models.SuccessResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.UnauthorizedErrorResponse "Missing or invalid JWT token"
// @Failure 500 {object} models.ErrorResponse
// @Router /exchanges/{vhost}/{exchange}/publish [post]
// @Security BearerAuth
func PublishMessage(c *fiber.Ctx, b *broker.Broker) error {
	vhost := c.Params("vhost")
	if vhost == "" {
		vhost = "/" // default vhost
	}
	var request models.PublishMessageRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: err.Error(),
		})
	}
	exchange := c.Params("exchange")
	if exchange != "" {
		decoded, err := url.PathUnescape(exchange)
		if err == nil {
			exchange = decoded
		}
	}

	err := b.Management.PublishMessage(vhost, exchange, request)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error: err.Error(),
		})
	}
	return c.Status(fiber.StatusOK).JSON(models.SuccessResponse{
		Message: "Message published",
	})
}

// GetMessage godoc
// @Summary Consume a message from a queue
// @Description Consume a message from the specified queue
// @Tags queues
// @Accept json
// @Produce json
// @Param queue path string true "Queue name"
// @Param vhost path string false "VHost name" default(/)
// @Success 200 {object} models.SuccessResponse
// @Failure 400 {object} models.ErrorResponse "Queue name is required"
// @Failure 401 {object} models.UnauthorizedErrorResponse "Missing or invalid JWT token"
// @Failure 404 {object} models.ErrorResponse "No messages in queue"
// @Failure 500 {object} models.ErrorResponse
// @Router /queues/{vhost}/{queue}/get [post]
// @Security BearerAuth
func GetMessage(c *fiber.Ctx, b *broker.Broker) error {
	vhost := c.Params("vhost")
	if vhost == "" {
		vhost = "/" // default vhost
	}
	queueName := c.Params("queue")
	if queueName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: "Queue name is required",
		})
	}
	var request models.GetMessageRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: "invalid request body: " + err.Error(),
		})
	}
	ackMode := models.AckType(request.AckMode)
	if ackMode == "" {
		ackMode = models.Ack
	}
	messageCount := request.MessageCount
	if messageCount <= 0 {
		messageCount = 1
	}
	msgs, err := b.Management.GetMessages(vhost, queueName, messageCount, ackMode)
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
