package api

import (
	"github.com/andrelcunha/ottermq/internal/core/models"
	"github.com/gofiber/fiber/v2"
	"github.com/rabbitmq/amqp091-go"
)

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
func PublishMessage(c *fiber.Ctx, ch *amqp091.Channel) error {
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

	msg := amqp091.Publishing{
		ContentType: "text/plain",
		Body:        []byte(request.Payload),
	}
	err := ch.Publish(
		exchange,
		request.RoutingKey,
		false, // mandatory
		false, // immediate
		msg,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error: err.Error(),
		})
	}
	return c.Status(fiber.StatusOK).JSON(models.SuccessResponse{
		Message: "Message published",
	})
}

// AckMessage godoc
// @Summary Acknowledge a message
// @Description Acknowledge a message with the specified ID
// @Tags messages
// @Accept json
// @Produce json
// @Param id path string true "Message ID"
// @Success 200 {object} models.SuccessResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.UnauthorizedErrorResponse "Missing or invalid JWT token"
// @Failure 500 {object} models.ErrorResponse
// @Router /messages/{id}/ack [post]
// @Security BearerAuth
func AckMessage(c *fiber.Ctx) error {
	// msgID := c.Params("id")
	// if msgID == "" {
	// 	return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
	// 		Error: "message ID is required",
	// 	})
	// }
	// command := fmt.Sprintf("ACK %s", msgID)
	// response, err := utils.SendCommand(command)
	// if err != nil {
	// 	return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
	// 		Error: err.Error(),
	// 	})
	// }

	// var commandResponse api.CommandResponse
	// if err := json.Unmarshal([]byte(response), &commandResponse); err != nil {
	// 	return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
	// 		Error: "failed to parse response",
	// 	})
	// }

	// if commandResponse.Status == "ERROR" {
	// 	return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
	// 		Error: commandResponse.Message,
	// 	})
	// } else {
	// 	return c.Status(fiber.StatusOK).JSON(models.SuccessResponse{
	// 		Message: commandResponse.Message,
	// 	})
	// }
	return nil // just to make the compiler happy
}
