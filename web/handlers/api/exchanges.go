package api

import (
	"github.com/andrelcunha/ottermq/internal/core/broker"
	"github.com/andrelcunha/ottermq/internal/core/models"

	"github.com/gofiber/fiber/v2"
)

// ListExchanges godoc
// @Summary List all exchanges
// @Description Get a list of all exchanges
// @Tags exchanges
// @Accept json
// @Produce json
// @Success 200 {object} models.ExchangeListResponse
// @Failure 401 {object} models.UnauthorizedErrorResponse "Missing or invalid JWT token"
// @Failure 500 {object} models.ErrorResponse "Failed to list exchanges"
// @Router /exchanges [get]
// @Security BearerAuth
func ListExchanges(c *fiber.Ctx, b *broker.Broker) error {
	exchanges, err := b.Management.ListExchanges("/")
	if err != nil || exchanges == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error: "Failed to list exchanges",
		})
	}
	return c.Status(fiber.StatusOK).JSON(models.ExchangeListResponse{
		Exchanges: exchanges,
	})
}

// CreateExchange godoc
// @Summary Create a new exchange
// @Description Create a new exchange with the specified name and type
// @Tags exchanges
// @Accept json
// @Produce json
// @Param exchange body models.CreateExchangeRequest true "Exchange to create"
// @Success 200 {object} models.SuccessResponse "Exchange created successfully"
// @Failure 400 {object} models.ErrorResponse "Invalid or malformed request body"
// @Failure 401 {object} models.UnauthorizedErrorResponse "Missing or invalid JWT token"
// @Failure 500 {object} models.ErrorResponse
// @Router /exchanges [post]
// @Security BearerAuth
func CreateExchange(c *fiber.Ctx, b *broker.Broker) error {
	var request models.CreateExchangeRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: "Invalid or malformed request body: " + err.Error(),
		})
	}
	exchangeDto := models.CreateExchangeRequest{
		VHost:        "/", // Assuming a default vhost for simplicity
		ExchangeName: request.ExchangeName,
		ExchangeType: request.ExchangeType,
		Durable:      request.Durable,
		AutoDelete:   request.AutoDelete,
		Internal:     request.Internal,
		Arguments:    request.Arguments,
	}

	_, err := b.Management.CreateExchange(exchangeDto)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error: err.Error(),
		})
	}
	return c.Status(fiber.StatusOK).JSON(models.SuccessResponse{
		Message: "Exchange created successfully",
	})
}

// DeleteExchange godoc
// @Summary Delete an exchange
// @Description Delete an exchange with the specified name
// @Tags exchanges
// @Accept json
// @Produce json
// @Param exchange path string true "Exchange name"
// @Success 204 {object} nil
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.UnauthorizedErrorResponse "Missing or invalid JWT token"
// @Failure 500 {object} models.ErrorResponse
// @Router /exchanges/{exchange} [delete]
// @Security BearerAuth
func DeleteExchange(c *fiber.Ctx, b *broker.Broker) error {
	exchangeName := c.Params("exchange")
	if exchangeName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: "Exchange name is required",
		})
	}

	err := b.Management.DeleteExchange("/", exchangeName, false)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error: err.Error(),
		})
	}

	return c.Status(fiber.StatusNoContent).Send(nil)
}

// PublishMessage godoc
// @Summary Publish a message to an exchange
// @Description Publish a message to the specified exchange with a routing key
// @Tags messages
// @Accept json
// @Produce json
// @Param message body models.PublishMessageRequest true "Message details"
// @Success 200 {object} models.SuccessResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.UnauthorizedErrorResponse "Missing or invalid JWT token"
// @Failure 500 {object} models.ErrorResponse
// @Router /messages [post]
// @Security BearerAuth
func PublishMessage(c *fiber.Ctx, b *broker.Broker) error {
	var request models.PublishMessageRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: err.Error(),
		})
	}
	exchange := request.ExchangeName
	if exchange == "(AMQP default)" {
		exchange = ""
	}

	err := b.Management.PublishMessage(request)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error: err.Error(),
		})
	}
	return c.Status(fiber.StatusOK).JSON(models.SuccessResponse{
		Message: "Message published",
	})
}
