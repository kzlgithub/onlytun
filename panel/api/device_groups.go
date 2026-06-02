package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	paneldb "github.com/onlytun/panel/db"
	"github.com/onlytun/panel/service"
)

type deviceGroupRuleCreateResponse struct {
	paneldb.DeviceGroupRule
	ConflictRule *paneldb.DeviceGroupRule `json:"conflict_rule,omitempty"`
}

func (h *Handler) ListMachineGroups(c *gin.Context) {
	groups, err := h.Groups.ListGroups(c.Query("role"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"groups": groups})
}

func (h *Handler) CreateMachineGroup(c *gin.Context) {
	var input service.MachineGroupInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	group, err := h.Groups.CreateGroup(input)
	if err != nil {
		writeGroupError(c, err)
		return
	}
	c.JSON(http.StatusCreated, group)
}

func (h *Handler) UpdateMachineGroup(c *gin.Context) {
	var input service.MachineGroupInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	group, err := h.Groups.UpdateGroup(c.Param("id"), input)
	if err != nil {
		writeGroupError(c, err)
		return
	}
	c.JSON(http.StatusOK, group)
}

func (h *Handler) DeleteMachineGroup(c *gin.Context) {
	if err := h.Groups.DeleteGroup(c.Param("id")); err != nil {
		writeGroupError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) SetMachineGroupMembers(c *gin.Context) {
	var input service.GroupMembersInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	group, err := h.Groups.SetGroupMembers(c.Param("id"), input)
	if err != nil {
		writeGroupError(c, err)
		return
	}
	c.JSON(http.StatusOK, group)
}

func (h *Handler) ListDeviceGroupRules(c *gin.Context) {
	rules, err := h.Groups.ListDeviceGroupRules()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ids := make([]string, 0, len(rules))
	for _, rule := range rules {
		ids = append(ids, rule.ID)
	}
	statsMap, err := h.Stats.LatestStatsForRules(ids)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	totalsMap, err := h.Rules.TotalTrafficForRules(ids)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	todayTotalsMap, err := h.Stats.TodayTotalsForRules(ids, time.Now())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	views, err := h.Groups.BuildDeviceGroupRuleViews(rules, statsMap, totalsMap, todayTotalsMap)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"rules": views})
}

func (h *Handler) CreateDeviceGroupRule(c *gin.Context) {
	var input service.DeviceGroupRuleInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	rule, conflict, err := h.Groups.CreateDeviceGroupRule(input)
	if err != nil {
		writeGroupError(c, err)
		return
	}
	c.JSON(http.StatusCreated, deviceGroupRuleCreateResponse{
		DeviceGroupRule: *rule,
		ConflictRule:    conflict,
	})
}

func (h *Handler) UpdateDeviceGroupRule(c *gin.Context) {
	var input service.DeviceGroupRuleInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	rule, err := h.Groups.UpdateDeviceGroupRule(c.Param("id"), input)
	if err != nil {
		writeGroupError(c, err)
		return
	}
	c.JSON(http.StatusOK, rule)
}

func (h *Handler) DeleteDeviceGroupRule(c *gin.Context) {
	if err := h.Groups.DeleteDeviceGroupRule(c.Param("id")); err != nil {
		writeGroupError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) ToggleDeviceGroupRule(c *gin.Context) {
	rule, err := h.Groups.ToggleDeviceGroupRule(c.Param("id"))
	if err != nil {
		writeGroupError(c, err)
		return
	}
	c.JSON(http.StatusOK, rule)
}

func writeGroupError(c *gin.Context, err error) {
	body := gin.H{"error": err.Error()}
	var conflict *service.DeviceGroupRulePortConflictError
	if errors.As(err, &conflict) {
		body["conflict_rule"] = conflict.Rule
	}
	c.JSON(groupErrorStatus(err), body)
}

func groupErrorStatus(err error) int {
	switch {
	case errors.Is(err, service.ErrGroupNotFound),
		errors.Is(err, service.ErrGroupRuleNotFound):
		return http.StatusNotFound
	case errors.Is(err, service.ErrInvalidRole),
		errors.Is(err, service.ErrInvalidMachine),
		errors.Is(err, service.ErrInvalidProtocol),
		errors.Is(err, service.ErrInvalidTarget),
		errors.Is(err, service.ErrInvalidPort),
		errors.Is(err, service.ErrInvalidTraffic),
		errors.Is(err, service.ErrGroupNameRequired):
		return http.StatusBadRequest
	case errors.Is(err, service.ErrGroupHasRules),
		errors.Is(err, service.ErrGroupRulePortConflict):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
