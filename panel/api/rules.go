package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	paneldb "github.com/onlytun/panel/db"
	"github.com/onlytun/panel/service"
)

type ruleCreateResponse struct {
	paneldb.ForwardRule
	ConflictRule *paneldb.ForwardRule `json:"conflict_rule,omitempty"`
}

func (h *Handler) ListRules(c *gin.Context) {
	rules, err := h.Rules.ListRules()
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

	items := make([]service.RuleView, 0, len(rules))
	for _, rule := range rules {
		total := totalsMap[rule.ID]
		items = append(items, service.RuleView{
			ForwardRule:   rule,
			RealtimeStat:  statsMap[rule.ID],
			TotalBytes:    total,
			TodayBytes:    todayTotalsMap[rule.ID],
			LimitExceeded: rule.TrafficLimitBytes > 0 && total >= rule.TrafficLimitBytes,
		})
	}

	c.JSON(http.StatusOK, gin.H{"rules": items})
}

func (h *Handler) CreateRule(c *gin.Context) {
	var input service.RuleInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	rule, conflictRule, err := h.Rules.CreateRule(input)
	if err != nil {
		writeRuleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, ruleCreateResponse{
		ForwardRule:  *rule,
		ConflictRule: conflictRule,
	})
}

func (h *Handler) UpdateRule(c *gin.Context) {
	var input service.RuleInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	rule, err := h.Rules.UpdateRule(c.Param("id"), input)
	if err != nil {
		writeRuleError(c, err)
		return
	}
	c.JSON(http.StatusOK, rule)
}

func (h *Handler) DeleteRule(c *gin.Context) {
	if err := h.Rules.DeleteRule(c.Param("id")); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrRuleNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) ToggleRule(c *gin.Context) {
	rule, err := h.Rules.ToggleRule(c.Param("id"))
	if err != nil {
		writeRuleError(c, err)
		return
	}
	c.JSON(http.StatusOK, rule)
}

func writeRuleError(c *gin.Context, err error) {
	body := gin.H{"error": err.Error()}
	var conflict *service.RulePortConflictError
	if errors.As(err, &conflict) {
		body["conflict_rule"] = conflict.Rule
	}
	c.JSON(ruleErrorStatus(err), body)
}

func ruleErrorStatus(err error) int {
	switch {
	case errors.Is(err, service.ErrRuleNotFound):
		return http.StatusNotFound
	case errors.Is(err, service.ErrInvalidMachine),
		errors.Is(err, service.ErrInvalidProtocol),
		errors.Is(err, service.ErrInvalidTarget):
		return http.StatusBadRequest
	case errors.Is(err, service.ErrRulePortConflict):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
