package api

import (
	"strings"

	"github.com/ottermq/ottermq/internal/core/broker"
	"github.com/ottermq/ottermq/internal/core/models"
	"github.com/ottermq/ottermq/internal/persistdb"
	"github.com/gofiber/fiber/v2"
)

// mandatoryExchanges are auto-created per vhost and must not appear in exports.
var mandatoryExchanges = map[string]bool{
	"amq.default": true,
	"amq.topic":   true,
	"amq.direct":  true,
	"amq.fanout":  true,
	"":            true,
}

// ExportDefinitions godoc
// @Summary Export all broker definitions
// @Description Export vhosts, users, permissions, exchanges, queues, and bindings as JSON.
// @Tags definitions
// @Produce json
// @Success 200 {object} models.DefinitionsDTO
// @Failure 401 {object} models.UnauthorizedErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /definitions [get]
// @Security BearerAuth
func ExportDefinitions(c *fiber.Ctx, b *broker.Broker) error {
	defs, err := buildDefinitions(c, b, "")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Error: err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(defs)
}

// ExportVHostDefinitions godoc
// @Summary Export definitions for a single virtual host
// @Description Export exchanges, queues, and bindings scoped to one vhost.
// @Tags definitions
// @Produce json
// @Param vhost path string true "VHost name"
// @Success 200 {object} models.DefinitionsDTO
// @Failure 401 {object} models.UnauthorizedErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /definitions/{vhost} [get]
// @Security BearerAuth
func ExportVHostDefinitions(c *fiber.Ctx, b *broker.Broker) error {
	vhost := decodedParam(c, "vhost")
	defs, err := buildDefinitions(c, b, vhost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Error: err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(defs)
}

// ImportDefinitions godoc
// @Summary Import broker definitions
// @Description Import vhosts, users, permissions, exchanges, queues, and bindings from JSON. Existing resources are skipped.
// @Tags definitions
// @Accept json
// @Produce json
// @Param definitions body models.DefinitionsDTO true "Definitions to import"
// @Success 200 {object} models.DefinitionsImportResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.UnauthorizedErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /definitions [post]
// @Security BearerAuth
func ImportDefinitions(c *fiber.Ctx, b *broker.Broker) error {
	var defs models.DefinitionsDTO
	if err := c.BodyParser(&defs); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Error: "invalid JSON body"})
	}
	result, err := applyDefinitions(b, defs, "")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Error: err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(result)
}

// ImportVHostDefinitions godoc
// @Summary Import definitions scoped to a virtual host
// @Description Import exchanges, queues, and bindings into the specified vhost. Existing resources are skipped.
// @Tags definitions
// @Accept json
// @Produce json
// @Param vhost path string true "VHost name"
// @Param definitions body models.DefinitionsDTO true "Definitions to import"
// @Success 200 {object} models.DefinitionsImportResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.UnauthorizedErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /definitions/{vhost} [post]
// @Security BearerAuth
func ImportVHostDefinitions(c *fiber.Ctx, b *broker.Broker) error {
	vhost := decodedParam(c, "vhost")
	var defs models.DefinitionsDTO
	if err := c.BodyParser(&defs); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Error: "invalid JSON body"})
	}
	result, err := applyDefinitions(b, defs, vhost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Error: err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(result)
}

// buildDefinitions assembles a DefinitionsDTO. When scopeVHost is non-empty only
// exchanges, queues, and bindings belonging to that vhost are included.
func buildDefinitions(c *fiber.Ctx, b *broker.Broker, scopeVHost string) (*models.DefinitionsDTO, error) {
	overview, err := b.Management.GetOverview()
	if err != nil {
		return nil, err
	}

	defs := &models.DefinitionsDTO{
		OtterMQVersion: overview.BrokerDetails.Version,
	}

	// VHosts
	vhosts, err := b.Management.ListVHosts()
	if err != nil {
		return nil, err
	}
	for _, v := range vhosts {
		if scopeVHost == "" || v.Name == scopeVHost {
			defs.VHosts = append(defs.VHosts, models.VHostDefinition{Name: v.Name})
		}
	}

	// Users & permissions only in global export
	if scopeVHost == "" {
		users, err := persistdb.GetUsersWithHashes()
		if err != nil {
			return nil, err
		}
		for _, u := range users {
			role, err := persistdb.GetRoleByID(u.RoleID)
			if err != nil {
				return nil, err
			}
			defs.Users = append(defs.Users, models.UserDefinition{
				Name:         u.Username,
				PasswordHash: u.Password,
				Tags:         role.Name,
			})
		}

		perms, err := persistdb.ListAllPermissions()
		if err != nil {
			return nil, err
		}
		for _, p := range perms {
			defs.Permissions = append(defs.Permissions, models.PermissionDefinition{
				User:  p.Username,
				VHost: p.VHost,
			})
		}
	}

	// Exchanges
	exchanges, err := b.Management.ListExchanges()
	if err != nil {
		return nil, err
	}
	for _, ex := range exchanges {
		if mandatoryExchanges[ex.Name] {
			continue
		}
		if scopeVHost != "" && ex.VHost != scopeVHost {
			continue
		}
		defs.Exchanges = append(defs.Exchanges, models.ExchangeDefinition{
			Name:       ex.Name,
			VHost:      ex.VHost,
			Type:       ex.Type,
			Durable:    ex.Durable,
			AutoDelete: ex.AutoDelete,
			Internal:   ex.Internal,
			Arguments:  ex.Arguments,
		})
	}

	// Queues
	queues := b.Management.ListQueues()
	for _, q := range queues {
		if scopeVHost != "" && q.VHost != scopeVHost {
			continue
		}
		defs.Queues = append(defs.Queues, models.QueueDefinition{
			Name:       q.Name,
			VHost:      q.VHost,
			Durable:    q.Durable,
			AutoDelete: q.AutoDelete,
			Arguments:  q.Arguments,
		})
	}

	// Bindings — skip default-exchange bindings (they're implicit)
	bindings, err := b.Management.ListBindings()
	if err != nil {
		return nil, err
	}
	for _, bd := range bindings {
		if mandatoryExchanges[bd.Source] {
			continue
		}
		if scopeVHost != "" && bd.VHost != scopeVHost {
			continue
		}
		defs.Bindings = append(defs.Bindings, models.BindingDefinition{
			Source:          bd.Source,
			VHost:           bd.VHost,
			Destination:     bd.Destination,
			DestinationType: bd.DestinationType,
			RoutingKey:      bd.RoutingKey,
			Arguments:       bd.Arguments,
		})
	}

	return defs, nil
}

// applyDefinitions imports a DefinitionsDTO into the broker. When scopeVHost is
// non-empty, vhost/user/permission entries are ignored and only resource entries
// matching that vhost are applied.
func applyDefinitions(b *broker.Broker, defs models.DefinitionsDTO, scopeVHost string) (*models.DefinitionsImportResponse, error) {
	result := &models.DefinitionsImportResponse{}

	if scopeVHost == "" {
		// VHosts
		for _, v := range defs.VHosts {
			if err := b.Management.CreateVHost(v.Name); err != nil && !isAlreadyExists(err) {
				return nil, err
			} else if err == nil {
				result.VHosts++
			}
		}

		// Users
		for _, u := range defs.Users {
			roleID := 2 // default: "user"
			switch strings.ToLower(u.Tags) {
			case "admin", "administrator":
				roleID = 1
			case "guest":
				roleID = 3
			}
			if err := persistdb.AddUserWithHash(u.Name, u.PasswordHash, roleID); err != nil {
				return nil, err
			}
			// AddUserWithHash uses INSERT OR IGNORE, count only when a new row was created.
			// We optimistically count +1; duplicates silently pass.
			result.Users++
		}

		// Permissions
		for _, p := range defs.Permissions {
			if err := persistdb.GrantVHostAccess(p.User, p.VHost); err != nil {
				return nil, err
			}
			result.Permissions++
		}
	}

	// Exchanges
	for _, ex := range defs.Exchanges {
		vh := ex.VHost
		if scopeVHost != "" {
			vh = scopeVHost
		}
		req := models.CreateExchangeRequest{
			ExchangeType: ex.Type,
			Durable:      ex.Durable,
			AutoDelete:   ex.AutoDelete,
			Arguments:    ex.Arguments,
		}
		if _, err := b.Management.CreateExchange(vh, ex.Name, req); err != nil && !isAlreadyExists(err) {
			return nil, err
		} else if err == nil {
			result.Exchanges++
		}
	}

	// Queues
	for _, q := range defs.Queues {
		vh := q.VHost
		if scopeVHost != "" {
			vh = scopeVHost
		}
		req := models.CreateQueueRequest{
			Durable:    q.Durable,
			AutoDelete: q.AutoDelete,
			Arguments:  q.Arguments,
		}
		if _, err := b.Management.CreateQueue(vh, q.Name, req); err != nil && !isAlreadyExists(err) {
			return nil, err
		} else if err == nil {
			result.Queues++
		}
	}

	// Bindings
	for _, bd := range defs.Bindings {
		vh := bd.VHost
		if scopeVHost != "" {
			vh = scopeVHost
		}
		req := models.CreateBindingRequest{
			VHost:       vh,
			Source:      bd.Source,
			Destination: bd.Destination,
			RoutingKey:  bd.RoutingKey,
			Arguments:   bd.Arguments,
		}
		if _, err := b.Management.CreateBinding(req); err != nil && !isAlreadyExists(err) {
			return nil, err
		} else if err == nil {
			result.Bindings++
		}
	}

	return result, nil
}

// isAlreadyExists reports whether an error indicates a resource already exists.
func isAlreadyExists(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "already exists") || strings.Contains(msg, "UNIQUE constraint")
}
