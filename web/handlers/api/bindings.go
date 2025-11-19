package api

import (
	"net/url"

	"github.com/andrelcunha/ottermq/internal/core/broker"
	"github.com/andrelcunha/ottermq/internal/core/models"

	"github.com/gofiber/fiber/v2"
	"github.com/rabbitmq/amqp091-go"
)

// BindQueue godoc
// @Summary Bind a queue to an exchange
// @Description Bind a queue to an exchange with the specified routing key
// @Tags bindings
// @Accept json
// @Produce json
// @Param binding body models.BindQueueRequest true "Binding details"
// @Success 200 {object} models.SuccessResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.UnauthorizedErrorResponse "Missing or invalid JWT token"
// @Failure 500 {object} models.ErrorResponse
// @Router /api/bindings [post]
// @Security BearerAuth
func BindQueue(c *fiber.Ctx, ch *amqp091.Channel) error {
	var request models.CreateBindingRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: err.Error(),
		})
	}

	err := ch.QueueBind(
		request.Destination,
		request.RoutingKey,
		request.Source,
		false, // noWait
		nil,   // arguments
	)
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

	bindings := b.ManagerApi.ListBindings("/", exchangeName)
	if bindings == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error: "failed to list bindings",
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
func DeleteBinding(c *fiber.Ctx) error {
	// var request models.DeleteBindingRequest
	// if err := c.BodyParser(&request); err != nil {
	// 	return c.Status(http.StatusBadRequest).JSON(models.ErrorResponse{
	// 		Error: err.Error(),
	// 	})
	// }
	// command := fmt.Sprintf("DELETE_BINDING %s %s %s",
	// 	request.ExchangeName,
	// 	request.QueueName,
	// 	request.RoutingKey)
	// response, err := utils.SendCommand(command)
	// if err != nil {
	// 	return c.Status(http.StatusInternalServerError).JSON(models.ErrorResponse{
	// 		Error: err.Error(),
	// 	})
	// }

	// var commandResponse api.CommandResponse
	// if err := json.Unmarshal([]byte(response), &commandResponse); err != nil {
	// 	return c.Status(http.StatusInternalServerError).JSON(models.ErrorResponse{
	// 		Error: "failed to parse response",
	// 	})
	// }

	// if commandResponse.Status == "ERROR" {
	// 	return c.Status(http.StatusInternalServerError).JSON(models.ErrorResponse{
	// 		Error: commandResponse.Message,
	// 	})
	// } else {
	// 	return c.Status(http.StatusNoContent).Send(nil)
	// }
	return nil // just to make the function compile
}
