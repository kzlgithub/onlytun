package reporter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/onlytun/agent/tunnel"
)

func TestReporterSendStatsPostsRegisteredSources(t *testing.T) {
	type capturedRequest struct {
		Path   string
		Auth   string
		Body   []byte
		Method string
	}

	var (
		mu   sync.Mutex
		reqs []capturedRequest
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		_ = r.Body.Close()

		mu.Lock()
		reqs = append(reqs, capturedRequest{
			Path:   r.URL.Path,
			Auth:   r.Header.Get("Authorization"),
			Body:   body,
			Method: r.Method,
		})
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	r := NewReporter(server.URL, "machine-1", "ingress", "token-1")
	r.client = server.Client()
	r.RegisterStatsSource("rule-b", func() Stats {
		return tunnel.Stats{BytesUp: 30, BytesDown: 40, ActiveConns: 2}
	})
	r.RegisterStatsSource("rule-a", func() Stats {
		return tunnel.Stats{BytesUp: 10, BytesDown: 20, ActiveConns: 1}
	})

	r.sendStats(context.Background())

	mu.Lock()
	defer mu.Unlock()
	if len(reqs) != 1 {
		t.Fatalf("expected one request, got %d", len(reqs))
	}
	if reqs[0].Method != http.MethodPost {
		t.Fatalf("expected POST request, got %s", reqs[0].Method)
	}
	if reqs[0].Path != "/api/agent/stats" {
		t.Fatalf("unexpected path %s", reqs[0].Path)
	}
	if reqs[0].Auth != "Bearer token-1" {
		t.Fatalf("unexpected auth header %q", reqs[0].Auth)
	}

	var payload statsPayload
	if err := json.Unmarshal(reqs[0].Body, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.MachineID != "machine-1" {
		t.Fatalf("unexpected machine id %q", payload.MachineID)
	}
	if len(payload.Stats) != 2 {
		t.Fatalf("expected 2 stats items, got %d", len(payload.Stats))
	}
	if payload.Stats[0].RuleID != "rule-a" || payload.Stats[1].RuleID != "rule-b" {
		t.Fatalf("stats items not sorted by rule id: %+v", payload.Stats)
	}
}

func TestReporterCollectStatsReportsDeltas(t *testing.T) {
	var up, down int64 = 100, 250
	r := NewReporter("http://127.0.0.1", "machine-1", "ingress", "token-1")
	r.RegisterStatsSource("rule-1", func() Stats {
		return tunnel.Stats{BytesUp: up, BytesDown: down, ActiveConns: 3}
	})

	first := r.collectStats()
	if len(first) != 1 {
		t.Fatalf("expected one stats item, got %d", len(first))
	}
	if first[0].BytesUp != 100 || first[0].BytesDown != 250 || first[0].ActiveConns != 3 {
		t.Fatalf("unexpected first stats item: %+v", first[0])
	}

	up = 175
	down = 400
	second := r.collectStats()
	if len(second) != 1 {
		t.Fatalf("expected one stats item, got %d", len(second))
	}
	if second[0].BytesUp != 75 || second[0].BytesDown != 150 || second[0].ActiveConns != 3 {
		t.Fatalf("unexpected delta stats item: %+v", second[0])
	}

	up = 20
	down = 30
	afterReset := r.collectStats()
	if afterReset[0].BytesUp != 20 || afterReset[0].BytesDown != 30 {
		t.Fatalf("expected counter reset to report current values, got %+v", afterReset[0])
	}
}

func TestReporterSendHeartbeatPostsPayload(t *testing.T) {
	type capturedRequest struct {
		Path string
		Auth string
		Body []byte
	}

	var (
		mu  sync.Mutex
		req capturedRequest
		ok  bool
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		_ = r.Body.Close()

		mu.Lock()
		req = capturedRequest{
			Path: r.URL.Path,
			Auth: r.Header.Get("Authorization"),
			Body: body,
		}
		ok = true
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	r := NewReporter(server.URL, "machine-2", "egress", "token-2")
	r.client = server.Client()
	r.sendHeartbeat(context.Background())

	mu.Lock()
	defer mu.Unlock()
	if !ok {
		t.Fatal("expected heartbeat request to be sent")
	}
	if req.Path != "/api/agent/heartbeat" {
		t.Fatalf("unexpected path %s", req.Path)
	}
	if req.Auth != "Bearer token-2" {
		t.Fatalf("unexpected auth header %q", req.Auth)
	}

	var payload heartbeatPayload
	if err := json.Unmarshal(req.Body, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.MachineID != "machine-2" {
		t.Fatalf("unexpected machine id %q", payload.MachineID)
	}
	if payload.Role != "egress" {
		t.Fatalf("unexpected role %q", payload.Role)
	}
	if payload.CPUPercent < 0 {
		t.Fatalf("unexpected cpu percent %f", payload.CPUPercent)
	}
	if payload.MemPercent < 0 {
		t.Fatalf("unexpected mem percent %f", payload.MemPercent)
	}
}

func TestReporterPostJSONReturnsErrorOnBadStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "denied", http.StatusUnauthorized)
	}))
	defer server.Close()

	r := NewReporter(server.URL, "machine-3", "ingress", "token-3")
	r.client = server.Client()

	err := r.postJSON(context.Background(), "/api/agent/stats", statsPayload{
		MachineID: "machine-3",
	})
	if err == nil {
		t.Fatal("expected postJSON to fail on non-2xx status")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected status code in error, got %v", err)
	}
}
