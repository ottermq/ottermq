package api

import (
	"net/url"

	"github.com/andrelcunha/ottermq/internal/core/broker"
	"github.com/andrelcunha/ottermq/internal/core/models"
	"github.com/gofiber/fiber/v2"
)

// ListVHosts godoc
// @Summary List all virtual hosts
// @Description Get a list of all virtual hosts
// @Tags vhosts
// @Produce json
// @Success 200 {object} models.VHostListResponse
// @Failure 401 {object} models.UnauthorizedErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /vhosts [get]
// @Security BearerAuth
func ListVHosts(c *fiber.Ctx, b *broker.Broker) error {
	vhosts, err := b.Management.ListVHosts()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error: "Failed to list vhosts",
		})
	}
	return c.Status(fiber.StatusOK).JSON(models.VHostListResponse{
		VHosts: vhosts,
	})
}

// GetVHost godoc
// @Summary Get a virtual host
// @Description Get details of a specific virtual host
// @Tags vhosts
// @Produce json
// @Param vhost path string true "VHost name"
// @Success 200 {object} models.VHostDTO
// @Failure 401 {object} models.UnauthorizedErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /vhosts/{vhost} [get]
// @Security BearerAuth
func GetVHost(c *fiber.Ctx, b *broker.Broker) error {
	name := decodedParam(c, "vhost")
	dto, err := b.Management.GetVHost(name)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error: "Failed to get vhost",
		})
	}
	if dto == nil {
		return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{
			Error: "VHost not found",
		})
	}
	return c.Status(fiber.StatusOK).JSON(dto)
}

// CreateVHost godoc
// @Summary Create a virtual host
// @Description Create a new virtual host
// @Tags vhosts
// @Produce json
// @Param vhost path string true "VHost name"
// @Success 201 {object} models.VHostDTO
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.UnauthorizedErrorResponse
// @Failure 409 {object} models.ErrorResponse
// @Router /vhosts/{vhost} [put]
// @Security BearerAuth
func CreateVHost(c *fiber.Ctx, b *broker.Broker) error {
	name := decodedParam(c, "vhost")
	if name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: "VHost name cannot be empty",
		})
	}
	if err := b.Management.CreateVHost(name); err != nil {
		return c.Status(fiber.StatusConflict).JSON(models.ErrorResponse{
			Error: err.Error(),
		})
	}
	dto, _ := b.Management.GetVHost(name)
	return c.Status(fiber.StatusCreated).JSON(dto)
}

// DeleteVHost godoc
// @Summary Delete a virtual host
// @Description Delete a virtual host and all its resources
// @Tags vhosts
// @Produce json
// @Param vhost path string true "VHost name"
// @Success 204
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.UnauthorizedErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /vhosts/{vhost} [delete]
// @Security BearerAuth
func DeleteVHost(c *fiber.Ctx, b *broker.Broker) error {
	name := decodedParam(c, "vhost")
	if err := b.Management.DeleteVHost(name); err != nil {
		status := fiber.StatusBadRequest
		if err.Error() == "vhost '"+name+"' not found" {
			status = fiber.StatusNotFound
		}
		return c.Status(status).JSON(models.ErrorResponse{
			Error: err.Error(),
		})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// decodedParam returns a URL-decoded path parameter.
func decodedParam(c *fiber.Ctx, key string) string {
	val := c.Params(key)
	if decoded, err := url.PathUnescape(val); err == nil {
		return decoded
	}
	return val
}
