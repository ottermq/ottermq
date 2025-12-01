package api

import (
	"net/url"
	"strconv"
	"strings"

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
	return listChannelsByVhost(b, "", c)
}

// ListConnectionChannels godoc
// @Summary List all channels for a specific connection
// @Description Get a list of all channels for a specific connection
// @Tags channels
// @Accept json
// @Produce json
// @Param name path string true "Connection ID"
// @Success 200 {object} models.ChannelListResponse
// @Failure 401 {object} models.UnauthorizedErrorResponse "Missing or invalid JWT token"
// @Failure 404 {object} models.ErrorResponse "Connection not found"
// @Failure 500 {object} models.ErrorResponse "Failed to list channels"
// @Router /connections/{name}/channels [get]
// @Security BearerAuth
func ListConnectionChannels(c *fiber.Ctx, b *broker.Broker) error {
	connectionID := c.Params("name")
	if connectionID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: "Connection ID is required",
		})
	} else {
		decoded, err := url.PathUnescape(connectionID)
		if err == nil {
			connectionID = decoded
		}
	}
	channels, err := b.Management.ListConnectionChannels(connectionID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{
				Error: err.Error(),
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error: "Failed to get channels: " + err.Error(),
		})
	}
	return c.Status(fiber.StatusOK).JSON(models.ChannelListResponse{
		Channels: channels,
	})
}

// ListChannelsByVhost godoc
// @Summary List channels by VHost
// @Description Get a list of all channels for a specific VHost
// @Tags channels
// @Accept json
// @Produce json
// @Param vhost path string true "VHost name" default(/)
// @Success 200 {object} models.ChannelListResponse
// @Failure 400 {object} models.ErrorResponse "Invalid or malformed request body"
// @Failure 404 {object} models.ErrorResponse "VHost not found"
// @Failure 401 {object} models.UnauthorizedErrorResponse "Missing or invalid JWT token"
// @Failure 500 {object} models.ErrorResponse "Failed to get channels"
// @Router /channels/{vhost} [get]
// @Security BearerAuth
func ListChannelsByVhost(c *fiber.Ctx, b *broker.Broker) error {
	vhost := c.Params("vhost")
	if vhost != "" {
		decoded, err := url.PathUnescape(vhost)
		if err == nil {
			vhost = decoded
		}
	}
	return listChannelsByVhost(b, vhost, c)
}

func listChannelsByVhost(b *broker.Broker, vhost string, c *fiber.Ctx) error {
	channels, err := b.Management.ListChannels(vhost)
	if err != nil {
		// TODO: improve error handling by using custom error types
		if strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{
				Error: err.Error(),
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error: "Failed to get channels: " + err.Error(),
		})
	}
	return c.Status(fiber.StatusOK).JSON(models.ChannelListResponse{
		Channels: channels,
	})
}

// GetChannel godoc
// @Summary Get channel details
// @Description Get details of a specific channel by its ID
// @Tags channels
// @Accept json
// @Produce json
// @Param name path string true "Connection ID"
// @Param channel path string true "Channel ID"
// @Success 200 {object} models.ChannelInfoDTO
// @Failure 400 {object} models.ErrorResponse "Channel ID is required"
// @Failure 401 {object} models.UnauthorizedErrorResponse "Missing or invalid JWT token"
// @Failure 404 {object} models.ErrorResponse "Channel not found"
// @Failure 500 {object} models.ErrorResponse "Failed to get channel"
// @Router /connections/{name}/channels/{channel} [get]
// @Security BearerAuth
func GetChannel(c *fiber.Ctx, b *broker.Broker) error {
	connectionID := c.Params("name")
	if connectionID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: "Connection ID is required",
		})
	} else {
		decoded, err := url.PathUnescape(connectionID)
		if err == nil {
			connectionID = decoded
		}
	}
	chanParam := c.Params("channel")
	var channelNumber uint16
	if chanParam == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: "Channel ID is required",
		})
	} else {
		// channel id must be an unsigned integer
		decoded, err := url.PathUnescape(chanParam)
		if err == nil {
			channelNumber64, err := strconv.ParseUint(decoded, 10, 16)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
					Error: "Channel ID must be a valid integer",
				})
			}
			channelNumber = uint16(channelNumber64)
		}
	}
	channel, err := b.Management.GetChannel(connectionID, channelNumber)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error: "Failed to get channel: " + err.Error(),
		})
	}
	return c.Status(fiber.StatusOK).JSON(channel)
}
