package api

import (
	"fmt"
	"net"
	"time"

	"github.com/ottermq/ottermq/internal/core/broker"
	"github.com/ottermq/ottermq/internal/core/models"
	"github.com/gofiber/fiber/v2"
)

// CheckAlarms godoc
// @Summary Check for broker-wide alarms
// @Description Returns ok when no broker-wide alarms are raised. OtterMQ does not yet implement alarms, so this always returns ok.
// @Tags health
// @Produce json
// @Success 200 {object} models.HealthCheckResponse
// @Failure 401 {object} models.UnauthorizedErrorResponse
// @Router /health/checks/alarms [get]
// @Security BearerAuth
func CheckAlarms(c *fiber.Ctx, b *broker.Broker) error {
	return c.Status(fiber.StatusOK).JSON(models.HealthCheckResponse{Status: "ok"})
}

// CheckLocalAlarms godoc
// @Summary Check for local node alarms
// @Description Returns ok when no local node alarms are raised. OtterMQ does not yet implement alarms, so this always returns ok.
// @Tags health
// @Produce json
// @Success 200 {object} models.HealthCheckResponse
// @Failure 401 {object} models.UnauthorizedErrorResponse
// @Router /health/checks/local-alarms [get]
// @Security BearerAuth
func CheckLocalAlarms(c *fiber.Ctx, b *broker.Broker) error {
	return c.Status(fiber.StatusOK).JSON(models.HealthCheckResponse{Status: "ok"})
}

// CheckPortListener godoc
// @Summary Check if a port is being listened on
// @Description Returns ok when the broker is listening on the specified port.
// @Tags health
// @Produce json
// @Param port path string true "Port number to check"
// @Success 200 {object} models.HealthCheckResponse
// @Failure 401 {object} models.UnauthorizedErrorResponse
// @Failure 503 {object} models.HealthCheckResponse
// @Router /health/checks/port-listener/{port} [get]
// @Security BearerAuth
func CheckPortListener(c *fiber.Ctx, b *broker.Broker) error {
	port := c.Params("port")
	addr := fmt.Sprintf("localhost:%s", port)

	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(models.HealthCheckResponse{
			Status: "failed",
			Reason: fmt.Sprintf("nothing is listening on port %s", port),
		})
	}
	conn.Close()
	return c.Status(fiber.StatusOK).JSON(models.HealthCheckResponse{Status: "ok"})
}

// CheckVirtualHosts godoc
// @Summary Check that all virtual hosts are running
// @Description Returns ok when all declared virtual hosts are initialised and operational.
// @Tags health
// @Produce json
// @Success 200 {object} models.HealthCheckResponse
// @Failure 401 {object} models.UnauthorizedErrorResponse
// @Failure 503 {object} models.HealthCheckResponse
// @Router /health/checks/virtual-hosts [get]
// @Security BearerAuth
func CheckVirtualHosts(c *fiber.Ctx, b *broker.Broker) error {
	vhosts, err := b.Management.ListVHosts()
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(models.HealthCheckResponse{
			Status: "failed",
			Reason: "could not retrieve virtual hosts",
		})
	}
	for _, v := range vhosts {
		if b.VHosts[v.Name] == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(models.HealthCheckResponse{
				Status: "failed",
				Reason: fmt.Sprintf("virtual host '%s' is not initialised", v.Name),
			})
		}
	}
	return c.Status(fiber.StatusOK).JSON(models.HealthCheckResponse{Status: "ok"})
}

// CheckReady godoc
// @Summary Check if the broker is ready to serve clients
// @Description Returns ok when the broker is up and accepting new connections.
// @Tags health
// @Produce json
// @Success 200 {object} models.HealthCheckResponse
// @Failure 401 {object} models.UnauthorizedErrorResponse
// @Failure 503 {object} models.HealthCheckResponse
// @Router /health/checks/ready [get]
// @Security BearerAuth
func CheckReady(c *fiber.Ctx, b *broker.Broker) error {
	if b.ShuttingDown.Load() {
		return c.Status(fiber.StatusServiceUnavailable).JSON(models.HealthCheckResponse{
			Status: "failed",
			Reason: "broker is shutting down",
		})
	}
	return c.Status(fiber.StatusOK).JSON(models.HealthCheckResponse{Status: "ok"})
}
