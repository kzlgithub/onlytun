package api

import (
	"net/http"
	"testing"

	"github.com/onlytun/panel/service"
)

func TestRuleErrorStatusMapsValidationErrorsToBadRequest(t *testing.T) {
	tests := []error{
		service.ErrInvalidMachine,
		service.ErrInvalidPort,
		service.ErrInvalidProtocol,
		service.ErrInvalidTarget,
		service.ErrInvalidTraffic,
	}

	for _, err := range tests {
		if got := ruleErrorStatus(err); got != http.StatusBadRequest {
			t.Fatalf("expected %v to map to %d, got %d", err, http.StatusBadRequest, got)
		}
	}
}
