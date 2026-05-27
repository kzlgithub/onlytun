package api

import (
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

var PanelVersion = "dev"

func (h *Handler) GetPanelVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"version": PanelVersion})
}

func (h *Handler) UpdatePanel(c *gin.Context) {
	if runtime.GOOS != "linux" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "panel update is only supported on linux systemd hosts"})
		return
	}

	script := `
set -eu
LOG=/var/log/onlytun-panel-update.log
if [ -f /root/install-panel.sh ]; then
  bash /root/install-panel.sh --update >>"$LOG" 2>&1
else
  bash <(curl -fsSL https://raw.githubusercontent.com/kzlgithub/onlytun/main/scripts/install-panel.sh) --update >>"$LOG" 2>&1
fi
`
	unit := fmt.Sprintf("onlytun-panel-update-%d", time.Now().UnixNano())
	runScript := "/bin/bash -c " + shellQuote(script)
	command := "systemd-run --unit=" + shellQuote(unit) + " --property=Type=oneshot " + runScript + " >/dev/null 2>&1 || " +
		"nohup /bin/bash -c " + shellQuote("sleep 1\n"+script) + " >/dev/null 2>&1 &"
	if err := exec.Command("/bin/bash", "-c", command).Start(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"ok":      true,
		"message": "panel update scheduled",
	})
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}
