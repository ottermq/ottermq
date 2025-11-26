package api

import (
	"net/url"

	"github.com/andrelcunha/ottermq/internal/core/broker"
	"github.com/andrelcunha/ottermq/internal/core/models"

	"github.com/gofiber/fiber/v2"
)

// BindQueue godoc
// @Summary Bind a queue to an exchange
// @Description Bind a queue to an exchange with the specified routing key
// @Tags bindings
// @Accept json
// @Produce json
// @Param binding body models.CreateBindingRequest true "Binding to create"
// @Success 200 {object} models.SuccessResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.UnauthorizedErrorResponse "Missing or invalid JWT token"
// @Failure 500 {object} models.ErrorResponse
// @Router /bindings [post]
// @Security BearerAuth
func BindQueue(c *fiber.Ctx, b *broker.Broker) error {
	var request models.CreateBindingRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: err.Error(),
		})
	}
	if request.VHost == "" {
		request.VHost = "/"
	}

	_, err := b.Management.CreateBinding(request)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error: err.Error(),
		})
	}
	return c.Status(fiber.StatusOK).JSON(models.SuccessResponse{
		Message: "Queue bound to exchange",
	})

}

// ListBindings godoc
// @Summary List all bindings for an exchange
// @Description Get a list of all bindings for the specified exchange
// @Tags bindings
// @Accept json
// @Produce json
// @Param exchange path string true "Exchange name"
// @Success 200 {object} models.BindingListResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.UnauthorizedErrorResponse "Missing or invalid JWT token"
// @Failure 500 {object} models.ErrorResponse
// @Router /bindings/{exchange} [get]
// @Security BearerAuth
func ListBindings(c *fiber.Ctx, b *broker.Broker) error {

	encodedExchangeName := c.Params("exchange")
	exchangeName, err := url.PathUnescape(encodedExchangeName)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: "Invalid exchange name",
		})
	}
	if exchangeName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: "Exchange name is required",
		})
	}

	bindings, err := b.Management.ListExchangeBindings("/", exchangeName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error: "failed to list bindings: " + err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(models.BindingListResponse{
		Bindings: bindings,
	})
}

// DeleteBinding godoc
// @Summary Delete a binding
// @Description Delete a binding from an exchange to a queue
// @Tags bindings
// @Accept json
// @Produce json
// @Param binding body models.DeleteBindingRequest true "Binding to delete"
// @Success 204 {object} nil
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.UnauthorizedErrorResponse "Missing or invalid JWT token"
// @Failure 500 {object} models.ErrorResponse
// @Router /bindings [delete]
// @Security BearerAuth
func DeleteBinding(c *fiber.Ctx, b *broker.Broker) error {
	var request models.DeleteBindingRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: err.Error(),
		})
	}
	if request.VHost == "" {
		request.VHost = "/"
	}
	err := b.Management.DeleteBinding(request)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error: err.Error(),
		})
	}

	return c.SendStatus(fiber.StatusNoContent)
}
