package api_admin

import (
	"github.com/ottermq/ottermq/internal/core/models"
	"github.com/ottermq/ottermq/internal/persistdb"
	"github.com/gofiber/fiber/v2"
)

// AddUser godoc
// @Summary Add a user
// @Description Add a user
// @Tags users
// @Accept json
// @Produce json
// @Param user body models.UserCreateRequest true "User details"
// @Success 200 {object} models.SuccessResponse "User added successfully"
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.UnauthorizedErrorResponse "Missing or invalid JWT token"
// @Failure 500 {object} models.ErrorResponse
// @Security ApiKeyAuth
// @Router /admin/users [post]
// @Security BearerAuth
func AddUser(c *fiber.Ctx) error {
	var user models.UserCreateRequest
	if err := c.BodyParser(&user); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Error: err.Error()})
	}
	if user.Password != user.ConfirmPassword {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Error: "Passwords do not match"})
	}
	err := persistdb.OpenDB()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Error: err.Error()})
	}
	defer persistdb.CloseDB()
	err = persistdb.AddUser(user.ToPersist())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Error: err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(models.SuccessResponse{Message: "User added successfully"})
}

// GetUsers godoc
// @Summary Get all users
// @Description Get all users
// @Tags users
// @Accept json
// @Produce json
// @Success 200 {object} models.UserListResponse
// @Failure 401 {object} models.UnauthorizedErrorResponse "Missing or invalid JWT token"
// @Failure 500 {object} models.ErrorResponse
// @Router /admin/users [get]
// @Security BearerAuth
func GetUsers(c *fiber.Ctx) error {
	err := persistdb.OpenDB()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Error: err.Error()})
	}
	defer persistdb.CloseDB()
	list, err := persistdb.GetUsers()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Error: err.Error()})
	}
	out := make([]models.UserSummary, 0, len(list))
	for _, u := range list {
		userdto, err := u.ToUserListDTO()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Error: err.Error()})
		}
		out = append(out, models.FromPersistUserListDTO(userdto))
	}
	return c.Status(fiber.StatusOK).JSON(models.UserListResponse{Users: out})
}

// Login godoc
// @Summary Login
// @Description Login
// @Tags auth
// @Accept json
// @Produce json
// @Param user body models.AuthRequest true "User details"
// @Success 200 {object} models.AuthResponse
// @Failure 401 {object} models.UnauthorizedErrorResponse "Invalid username or password"
// @Failure 500 {object} models.ErrorResponse
// @Router /login [post]
func Login(jwtSecret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req models.AuthRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Error: err.Error()})
		}
		err := persistdb.OpenDB()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Error: err.Error()})
		}
		defer persistdb.CloseDB()

		ok, err := persistdb.AuthenticateUser(req.Username, req.Password)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Error: err.Error()})
		}
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(models.UnauthorizedErrorResponse{Error: "Invalid username or password"})
		}
		// get user
		persistedUser, err := persistdb.GetUserByUsername(req.Username)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Error: err.Error()})
		}

		userdto, err := persistedUser.ToUserListDTO()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Error: err.Error()})
		}
		token, err := persistdb.GenerateJWTToken(userdto, jwtSecret)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Error: err.Error()})
		}
		return c.Status(fiber.StatusOK).JSON(models.AuthResponse{Token: token})
	}
}
