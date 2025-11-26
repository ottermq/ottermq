package api

import (
	"net/url"

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
	exchanges, err := b.Management.ListExchanges()
	if err != nil || exchanges == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error: "Failed to list exchanges",
		})
	}
	return c.Status(fiber.StatusOK).JSON(models.ExchangeListResponse{
		Exchanges: exchanges,
	})
}

// GetExchange godoc
// @Summary Get an exchange
// @Description Get an exchange by name
// @Tags exchanges
// @Accept json
// @Produce json
// @Param vhost path string false "VHost name" default(/)
// @Param exchange path string true "Exchange name"
// @Success 200 {object} models.ExchangeDTO
// @Failure 401 {object} models.UnauthorizedErrorResponse "Missing or invalid JWT token"
// @Failure 500 {object} models.ErrorResponse "Failed to get exchange"
// @Router /exchanges/{vhost}/{exchange} [get]
// @Security BearerAuth
func GetExchange(c *fiber.Ctx, b *broker.Broker) error {
	vhost := c.Params("vhost")
	if vhost == "" {
		vhost = "/" // default vhost
	} else {
		decoded, err := url.PathUnescape(vhost)
		if err == nil {
			vhost = decoded
		}
	}
	exchangeName := c.Params("exchange")
	if exchangeName != "" {
		decoded, err := url.PathUnescape(exchangeName)
		if err == nil {
			exchangeName = decoded
		}
	}
	exchange, err := b.Management.GetExchange(vhost, exchangeName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error: err.Error(),
		})
	}
	return c.Status(fiber.StatusOK).JSON(exchange)
}

// CreateExchange godoc
// @Summary Create a new exchange
// @Description Create a new exchange with the specified name and type
// @Tags exchanges
// @Accept json
// @Produce json
// @Param vhost path string false "VHost name" default(/)
// @Param exchange path string true "Exchange name"
// @Param exchange body models.CreateExchangeRequest true "Exchange to create"
// @Success 200 {object} models.SuccessResponse "Exchange created successfully"
// @Failure 400 {object} models.ErrorResponse "Invalid or malformed request body"
// @Failure 401 {object} models.UnauthorizedErrorResponse "Missing or invalid JWT token"
// @Failure 500 {object} models.ErrorResponse
// @Router /exchanges/{vhost}/{exchange} [post]
// @Security BearerAuth
func CreateExchange(c *fiber.Ctx, b *broker.Broker) error {
	vhost := c.Params("vhost")
	if vhost == "" {
		vhost = "/" // default vhost
	} else {
		decoded, err := url.PathUnescape(vhost)
		if err == nil {
			vhost = decoded
		}
	}
	exchangeName := c.Params("exchange")
	if exchangeName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: "Exchange name is required",
		})
	} else {
		decoded, err := url.PathUnescape(exchangeName)
		if err == nil {
			exchangeName = decoded
		}
	}
	var request models.CreateExchangeRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: "Invalid or malformed request body: " + err.Error(),
		})
	}
	_, err := b.Management.CreateExchange(vhost, exchangeName, request)
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
// @Param vhost path string false "VHost name" default(/)
// @Param exchange path string true "Exchange name"
// @Success 204 {object} nil
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.UnauthorizedErrorResponse "Missing or invalid JWT token"
// @Failure 500 {object} models.ErrorResponse
// @Router /exchanges/{vhost}/{exchange} [delete]
// @Security BearerAuth
func DeleteExchange(c *fiber.Ctx, b *broker.Broker) error {
	vhost := c.Params("vhost")
	if vhost == "" {
		vhost = "/" // default vhost
	} else {
		decoded, err := url.PathUnescape(vhost)
		if err == nil {
			vhost = decoded
		}
	}
	exchangeName := c.Params("exchange")
	if exchangeName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: "Exchange name is required",
		})
	}

	err := b.Management.DeleteExchange(vhost, exchangeName, false)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error: err.Error(),
		})
	}

	return c.Status(fiber.StatusNoContent).Send(nil)
}

// GetBindingsExchangeSource godoc
// @Summary Get bindings where the exchange is the source
// @Description Get all bindings where the specified exchange is the source
// @Tags exchanges
// @Accept json
// @Produce json
// @Param vhost path string false "VHost name" default(/)
// @Param exchange path string true "Exchange name"
// @Success 200 {object} models.BindingListResponse
// @Failure 401 {object} models.UnauthorizedErrorResponse "Missing or invalid JWT token"
// @Failure 500 {object} models.ErrorResponse "Failed to get bindings"
// @Router /exchanges/{vhost}/{exchange}/bindings/source [get]
// @Security BearerAuth
func GetBindingsExchangeSource(c *fiber.Ctx, b *broker.Broker) error {
	vhost := c.Params("vhost")
	if vhost == "" {
		vhost = "/" // default vhost
	} else {
		decoded, err := url.PathUnescape(vhost)
		if err == nil {
			vhost = decoded
		}
	}
	exchangeName := c.Params("exchange")
	if exchangeName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: "Exchange name is required",
		})
	} else {
		decoded, err := url.PathUnescape(exchangeName)
		if err == nil {
			exchangeName = decoded
		}
	}
	bindings, err := b.Management.ListExchangeBindings(vhost, exchangeName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error: err.Error(),
		})
	}
	return c.Status(fiber.StatusOK).JSON(models.BindingListResponse{
		Bindings: bindings,
	})
}
