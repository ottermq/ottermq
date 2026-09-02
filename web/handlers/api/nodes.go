package api

import (
	"runtime"

	"github.com/ottermq/ottermq/internal/core/broker"
	"github.com/ottermq/ottermq/internal/core/models"
	"github.com/gofiber/fiber/v2"
)

// ListNodes godoc
// @Summary List cluster nodes
// @Description Returns the list of nodes in the cluster. OtterMQ is single-node, so this always returns one entry.
// @Tags nodes
// @Produce json
// @Success 200 {array} models.OverviewNodeDetails
// @Failure 401 {object} models.UnauthorizedErrorResponse
// @Router /nodes [get]
// @Security BearerAuth
func ListNodes(c *fiber.Ctx, b *broker.Broker) error {
	nd := b.GetOverviewNodeDetails()
	return c.Status(fiber.StatusOK).JSON([]models.OverviewNodeDetails{nd})
}

// GetNode godoc
// @Summary Get node details
// @Description Returns details for a specific node by name (e.g. ottermq@hostname).
// @Tags nodes
// @Produce json
// @Param name path string true "Node name"
// @Success 200 {object} models.OverviewNodeDetails
// @Failure 401 {object} models.UnauthorizedErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /nodes/{name} [get]
// @Security BearerAuth
func GetNode(c *fiber.Ctx, b *broker.Broker) error {
	name := decodedParam(c, "name")
	nd := b.GetOverviewNodeDetails()
	if name != nd.Name {
		return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{
			Error: "node '" + name + "' not found",
		})
	}
	return c.Status(fiber.StatusOK).JSON(nd)
}

// GetNodeMemory godoc
// @Summary Get node memory breakdown
// @Description Returns a detailed memory usage breakdown for the specified node.
// @Tags nodes
// @Produce json
// @Param name path string true "Node name"
// @Success 200 {object} models.NodeMemoryDTO
// @Failure 401 {object} models.UnauthorizedErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /nodes/{name}/memory [get]
// @Security BearerAuth
func GetNodeMemory(c *fiber.Ctx, b *broker.Broker) error {
	name := decodedParam(c, "name")
	nd := b.GetOverviewNodeDetails()
	if name != nd.Name {
		return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{
			Error: "node '" + name + "' not found",
		})
	}

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	dto := models.NodeMemoryDTO{
		Memory: models.NodeMemoryBreakdown{
			Total:      nd.MemoryLimit,
			Used:       nd.MemoryUsage,
			HeapAlloc:  int(ms.HeapAlloc),
			HeapSys:    int(ms.HeapSys),
			HeapInUse:  int(ms.HeapInuse),
			HeapIdle:   int(ms.HeapIdle),
			StackInUse: int(ms.StackInuse),
			GCSys:      int(ms.GCSys),
			OtherSys:   int(ms.OtherSys),
		},
	}
	return c.Status(fiber.StatusOK).JSON(dto)
}
