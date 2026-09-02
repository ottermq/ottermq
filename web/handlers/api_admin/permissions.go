package api_admin

import (
	"github.com/gofiber/fiber/v2"
	"github.com/ottermq/ottermq/internal/core/models"
	"github.com/ottermq/ottermq/internal/persistdb"
)

// ListPermissions godoc
// @Summary List all vhost permissions
// @Description List every user-vhost access grant
// @Tags permissions
// @Produce json
// @Success 200 {object} models.PermissionListResponse
// @Failure 401 {object} models.UnauthorizedErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /admin/permissions [get]
// @Security BearerAuth
func ListPermissions(c *fiber.Ctx) error {
	perms, err := persistdb.ListAllPermissions()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Error: err.Error()})
	}
	out := make([]models.PermissionDTO, 0, len(perms))
	for _, p := range perms {
		out = append(out, models.PermissionDTO{Username: p.Username, VHost: p.VHost})
	}
	return c.Status(fiber.StatusOK).JSON(models.PermissionListResponse{Permissions: out})
}

// GetPermission godoc
// @Summary Get vhost permission for a user
// @Description Check whether a user has access to a specific vhost
// @Tags permissions
// @Produce json
// @Param vhost path string true "VHost name"
// @Param username path string true "Username"
// @Success 200 {object} models.PermissionDTO
// @Failure 401 {object} models.UnauthorizedErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /admin/permissions/{vhost}/{username} [get]
// @Security BearerAuth
func GetPermission(c *fiber.Ctx) error {
	vhost := c.Params("vhost")
	username := c.Params("username")
	ok, err := persistdb.HasVHostAccess(username, vhost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Error: err.Error()})
	}
	if !ok {
		return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{
			Error: "no permission found for user '" + username + "' on vhost '" + vhost + "'",
		})
	}
	return c.Status(fiber.StatusOK).JSON(models.PermissionDTO{Username: username, VHost: vhost})
}

// GrantPermission godoc
// @Summary Grant vhost access to a user
// @Description Allow a user to connect to the specified vhost
// @Tags permissions
// @Produce json
// @Param vhost path string true "VHost name"
// @Param username path string true "Username"
// @Success 201 {object} models.PermissionDTO
// @Failure 401 {object} models.UnauthorizedErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /admin/permissions/{vhost}/{username} [put]
// @Security BearerAuth
func GrantPermission(c *fiber.Ctx) error {
	vhost := c.Params("vhost")
	username := c.Params("username")
	if err := persistdb.GrantVHostAccess(username, vhost); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Error: err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(models.PermissionDTO{Username: username, VHost: vhost})
}

// RevokePermission godoc
// @Summary Revoke vhost access from a user
// @Description Remove a user's access to the specified vhost
// @Tags permissions
// @Produce json
// @Param vhost path string true "VHost name"
// @Param username path string true "Username"
// @Success 204
// @Failure 401 {object} models.UnauthorizedErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /admin/permissions/{vhost}/{username} [delete]
// @Security BearerAuth
func RevokePermission(c *fiber.Ctx) error {
	vhost := c.Params("vhost")
	username := c.Params("username")
	if err := persistdb.RevokeVHostAccess(username, vhost); err != nil {
		status := fiber.StatusInternalServerError
		if err.Error() == "no permission found for user '"+username+"' on vhost '"+vhost+"'" {
			status = fiber.StatusNotFound
		}
		return c.Status(status).JSON(models.ErrorResponse{Error: err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}
