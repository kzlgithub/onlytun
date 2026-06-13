package reporter

import (
	"math"
	"testing"
	"time"
)

func TestCalculateNetSpeed(t *testing.T) {
	up, down := calculateNetSpeed(1100, 2400, 1000, 2000, time.Second, true)
	if up != 100 || down != 400 {
		t.Fatalf("expected 100/400 Bps, got %.2f/%.2f", up, down)
	}

	up, down = calculateNetSpeed(1100, 2400, 0, 0, time.Second, false)
	if up != 0 || down != 0 {
		t.Fatalf("first sample must report zero speed, got %.2f/%.2f", up, down)
	}

	up, down = calculateNetSpeed(900, 2400, 1000, 2000, time.Second, true)
	if up != 0 || down != 0 {
		t.Fatalf("counter reset must report zero speed, got %.2f/%.2f", up, down)
	}
}

func TestSmoothNetSpeedUsesConfiguredAlpha(t *testing.T) {
	got := smoothNetSpeed(100, 20)
	want := 72.0
	if math.Abs(got-want) > 0.000001 {
		t.Fatalf("expected %.2f, got %.2f", want, got)
	}
}

func TestRoundBps(t *testing.T) {
	if got := roundBps(12.49); got != 12 {
		t.Fatalf("expected 12, got %d", got)
	}
	if got := roundBps(12.5); got != 13 {
		t.Fatalf("expected 13, got %d", got)
	}
	if got := roundBps(-1); got != 0 {
		t.Fatalf("expected 0, got %d", got)
	}
}
