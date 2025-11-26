package api

import (
	"strconv"

	"github.com/andrelcunha/ottermq/internal/core/broker"
	"github.com/andrelcunha/ottermq/internal/core/models"

	"github.com/gofiber/fiber/v2"
)

// ListChannels godoc
// @Summary List all channels
// @Description Get a list of all channels
// @Tags channels
// @Accept json
// @Produce json
// @Success 200 {object} models.ChannelListResponse
// @Failure 401 {object} models.UnauthorizedErrorResponse "Missing or invalid JWT token"
// @Failure 500 {object} models.ErrorResponse "Failed to list channels"
// @Router /channels [get]
// @Security BearerAuth
func ListChannels(c *fiber.Ctx, b *broker.Broker) error {
	channels, err := b.Management.ListChannels()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error: "Failed to list channels: " + err.Error(),
		})
	}
	return c.Status(fiber.StatusOK).JSON(models.ChannelListResponse{
		Channels: channels,
	})
}

// ListConnectionChannels godoc
// @Summary List all channels for a specific connection
// @Description Get a list of all channels for a specific connection
// @Tags channels
// @Accept json
// @Produce json
// @Success 200 {object} models.ChannelListResponse
// @Failure 401 {object} models.UnauthorizedErrorResponse "Missing or invalid JWT token"
// @Failure 500 {object} models.ErrorResponse "Failed to list channels"
// @Router /channels/connection [get]
// @Security BearerAuth
func ListConnectionChannels(c *fiber.Ctx, b *broker.Broker) error {
	// Assuming connection ID is passed as a query parameter
	connectionID := c.Query("connection_id")
	if connectionID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: "Connection ID is required",
		})
	}
	channels, err := b.Management.ListConnectionChannels(connectionID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error: "Failed to list channels: " + err.Error(),
		})
	}
	return c.Status(fiber.StatusOK).JSON(models.ChannelListResponse{
		Channels: channels,
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
func GetChannel(c *fiber.Ctx, b *broker.Broker) error {
	connectionID := c.Query("connection_id")
	if connectionID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: "Connection ID is required",
		})
	}
	// convert channel ID from string to uint16
	channelID, err := strconv.ParseUint(c.Params("channel_id"), 10, 16)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: "Channel ID is required",
		})
	}
	channel, err := b.Management.GetChannel(connectionID, uint16(channelID))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error: "Failed to get channel: " + err.Error(),
		})
	}
	return c.Status(fiber.StatusOK).JSON(channel)
}
