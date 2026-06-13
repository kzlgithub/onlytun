package reporter

import (
	"context"
	"log"
	"time"
)

const (
	netStatsInterval = time.Second
	netSpeedAlpha    = 0.65
)

type netStatsPayload struct {
	MachineID    string `json:"machine_id"`
	NetBytesUp   uint64 `json:"net_bytes_up"`
	NetBytesDown uint64 `json:"net_bytes_down"`
	NetUpBps     uint64 `json:"net_up_bps"`
	NetDownBps   uint64 `json:"net_down_bps"`
}

func (r *Reporter) runNetStatsLoop(ctx context.Context) {
	ticker := time.NewTicker(netStatsInterval)
	defer ticker.Stop()

	var lastUp uint64
	var lastDown uint64
	var lastSampleAt time.Time
	var smoothUp float64
	var smoothDown float64
	failures := 0

	send := func() {
		now := time.Now()
		bytesUp, bytesDown, err := sampleNetBytesWithError()
		if err != nil {
			failures++
			if failures == 1 || failures%30 == 0 {
				log.Printf("[WARN] reporter net stats sample failed: %v", err)
			}
			return
		}
		counterReset := !lastSampleAt.IsZero() && (bytesUp < lastUp || bytesDown < lastDown)
		upBps, downBps := calculateNetSpeed(bytesUp, bytesDown, lastUp, lastDown, now.Sub(lastSampleAt), !lastSampleAt.IsZero())
		if counterReset {
			smoothUp = 0
			smoothDown = 0
		} else {
			smoothUp = smoothNetSpeed(upBps, smoothUp)
			smoothDown = smoothNetSpeed(downBps, smoothDown)
		}

		payload := netStatsPayload{
			MachineID:    r.machineID,
			NetBytesUp:   bytesUp,
			NetBytesDown: bytesDown,
			NetUpBps:     roundBps(smoothUp),
			NetDownBps:   roundBps(smoothDown),
		}

		if err := r.postJSON(ctx, "/api/agent/net-stats", payload); err != nil {
			failures++
			if failures == 1 || failures%30 == 0 {
				log.Printf("[WARN] reporter net stats post failed: %v", err)
			}
		} else {
			failures = 0
		}

		lastUp = bytesUp
		lastDown = bytesDown
		lastSampleAt = now
	}

	send()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			send()
		}
	}
}

func calculateNetSpeed(currentUp, currentDown, lastUp, lastDown uint64, elapsed time.Duration, hasLast bool) (float64, float64) {
	if !hasLast || elapsed <= 0 || currentUp < lastUp || currentDown < lastDown {
		return 0, 0
	}
	seconds := elapsed.Seconds()
	return float64(currentUp-lastUp) / seconds, float64(currentDown-lastDown) / seconds
}

func smoothNetSpeed(current, previous float64) float64 {
	return current*netSpeedAlpha + previous*(1-netSpeedAlpha)
}

func roundBps(value float64) uint64 {
	if value <= 0 {
		return 0
	}
	return uint64(value + 0.5)
}
