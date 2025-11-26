package api

import (
	"github.com/andrelcunha/ottermq/internal/core/broker"
	"github.com/gofiber/fiber/v2"
)

// GetBasicBrokerInfo godoc
// @Summary Get basic broker information
// @Description Retrieve basic information about the message broker
// @Tags overview
// @Accept json
// @Produce json
// @Success 200 {object} models.OverviewBrokerDetails
// @Failure 500 {object} models.ErrorResponse "Failed to get broker information"
// @Router /overview/broker [get]
// @Security BearerAuth
func GetBasicBrokerInfo(c *fiber.Ctx, b *broker.Broker) error {
	info := b.Management.GetBrokerInfo()
	return c.Status(fiber.StatusOK).JSON(info)
}

// GetBasicBrokerInfo godoc
// @Summary Get basic broker information
// @Description Retrieve basic information about the message broker
// @Tags overview
// @Accept json
// @Produce json
// @Success 200 {object} models.OverviewDTO
// @Failure 500 {object} models.ErrorResponse "Failed to get broker information"
// @Router /overview [get]
func GetOverview(c *fiber.Ctx, b *broker.Broker) error {
	overview, err := b.Management.GetOverview()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get overview: " + err.Error(),
		})
	}
	return c.Status(fiber.StatusOK).JSON(overview)
}
