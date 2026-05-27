package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/onlytun/panel/service"
)

type updateMachineRequest struct {
	Name string `json:"name"`
}

func (h *Handler) ListMachines(c *gin.Context) {
	items, err := h.Machines.ListMachines()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"machines": items})
}

func (h *Handler) GenerateMachineToken(c *gin.Context) {
	record, err := h.Machines.GenerateInstallToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"token":      record.Token,
		"expires_at": record.ExpiresAt,
	})
}

func (h *Handler) UpdateMachine(c *gin.Context) {
	var req updateMachineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	machine, err := h.Machines.UpdateMachineName(c.Param("id"), req.Name)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, service.ErrMachineNameRequired):
			status = http.StatusBadRequest
		case errors.Is(err, service.ErrMachineNotFound):
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"machine": machine})
}

func (h *Handler) DeleteMachine(c *gin.Context) {
	if err := h.Machines.DeleteMachine(c.Param("id")); err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, service.ErrMachineHasRules):
			status = http.StatusBadRequest
		case errors.Is(err, service.ErrMachineNotFound):
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) RequestMachineUpdate(c *gin.Context) {
	task, err := h.Machines.RequestMachineUpdate(c.Param("id"))
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, service.ErrMachineNotFound):
			status = http.StatusNotFound
		case errors.Is(err, service.ErrMachineOffline):
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"task": task})
}

func (h *Handler) GetInstallScript(c *gin.Context) {
	scheme := "http"
	if forwarded := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")); forwarded != "" {
		scheme = forwarded
	} else if c.Request.TLS != nil {
		scheme = "https"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, c.Request.Host)

	payload, err := h.Machines.BuildInstallScripts(baseURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, payload)
}
