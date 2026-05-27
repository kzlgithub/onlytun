package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

func (h *Handler) GetPanelMetrics(c *gin.Context) {
	cpuValues, err := cpu.Percent(0, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	memStats, err := mem.VirtualMemory()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	diskPercent := 0.0
	if diskStats, err := disk.Usage("/"); err == nil {
		diskPercent = diskStats.UsedPercent
	}

	c.JSON(http.StatusOK, gin.H{
		"cpu_percent":  firstMetric(cpuValues),
		"mem_percent":  memStats.UsedPercent,
		"disk_percent": diskPercent,
	})
}

func firstMetric(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	return values[0]
}
