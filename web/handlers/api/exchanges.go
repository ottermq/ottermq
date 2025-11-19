package api

import (
	"github.com/andrelcunha/ottermq/internal/core/broker"
	"github.com/andrelcunha/ottermq/internal/core/models"

	"github.com/gofiber/fiber/v2"
	"github.com/rabbitmq/amqp091-go"
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
	exchanges := b.ManagerApi.ListExchanges()
	if exchanges == nil {
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
	exchangeDto := models.ExchangeDTO{
		VHost: "/", // Assuming a default vhost for simplicity
		Name:  request.ExchangeName,
		Type:  request.ExchangeType,
	}

	err := b.ManagerApi.CreateExchange(exchangeDto)
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
func DeleteExchange(c *fiber.Ctx, ch *amqp091.Channel) error {
	exchangeName := c.Params("exchange")
	if exchangeName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: "Exchange name is required",
		})
	}

	err := ch.ExchangeDelete(exchangeName, false, false)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error: err.Error(),
		})
	}

	return c.Status(fiber.StatusNoContent).Send(nil)
}
