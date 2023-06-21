package handlers

import (
	"github.com/ausaf007/uniswap-tracker/services"
	"github.com/gofiber/fiber/v2"
	log "github.com/sirupsen/logrus"
	"strconv"
)

type PoolHandler struct {
	service *services.TrackingService
}

func NewPoolHandler(service *services.TrackingService) *PoolHandler {
	return &PoolHandler{service: service}
}

func (h *PoolHandler) PoolDataHandler(c *fiber.Ctx) error {
	poolID, err := strconv.ParseUint(c.Params("pool_id"), 10, 64)
	if err != nil {
		log.Error("Invalid Pool ID: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid pool_id"})
	}
	block := c.Query("block", "latest")

	poolData, err := h.service.GetPoolData(uint(poolID), block)
	if err != nil {
		log.Error("Unable to fetch Pool Data: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(poolData)
}

func (h *PoolHandler) HistoricPoolDataHandler(c *fiber.Ctx) error {
	poolID, err := strconv.ParseUint(c.Params("pool_id"), 10, 64)
	if err != nil {
		log.Error("Invalid Pool ID: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid pool_id"})
	}

	poolData, err := h.service.GetHistoricPoolData(uint(poolID))
	if err != nil {
		log.Error("Unable to fetch Historic Pool Data: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(poolData)
}