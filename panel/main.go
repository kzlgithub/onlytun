package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/onlytun/panel/api"
	paneldb "github.com/onlytun/panel/db"
	"github.com/onlytun/panel/service"
)

const (
	defaultPort       = 8080
	defaultTunnelPort = 19999
	databasePath      = "/etc/onlytun/panel.db"
	offlineCheckEvery = 30 * time.Second
	offlineAfter      = 90 * time.Second
	requestTimeout    = 10 * time.Second
)

// Note: go:embed cannot reference ../web/dist directly.
// The release/build flow syncs root web/dist into panel/web/dist before compiling.
//go:embed all:web/dist
var webFS embed.FS

func main() {
	password := strings.TrimSpace(os.Getenv("ONLYTUN_PASSWORD"))
	if password == "" {
		log.Fatal("ONLYTUN_PASSWORD must be set")
	}

	port := envInt("ONLYTUN_PORT", defaultPort)
	tunnelPort := envInt("ONLYTUN_TUNNEL_PORT", defaultTunnelPort)

	gdb, err := paneldb.OpenDatabase(databasePath)
	if err != nil {
		log.Fatalf("open database failed: %v", err)
	}

	machineService := service.NewMachineService(gdb, tunnelPort)
	ruleService := service.NewRuleService(gdb, tunnelPort)
	statsService := service.NewStatsService(gdb)
	handler := api.NewHandler(machineService, ruleService, statsService, password)

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	if gin.Mode() != gin.ReleaseMode {
		router.Use(corsMiddleware())
	}

	apiGroup := router.Group("/api")
	apiGroup.POST("/login", handler.Login)

	admin := apiGroup.Group("/")
	admin.Use(handler.RequireAdminJWT())
	admin.GET("/machines", handler.ListMachines)
	admin.POST("/machines/generate-token", handler.GenerateMachineToken)
	admin.DELETE("/machines/:id", handler.DeleteMachine)
	admin.GET("/machines/install-script", handler.GetInstallScript)
	admin.GET("/rules", handler.ListRules)
	admin.POST("/rules", handler.CreateRule)
	admin.PUT("/rules/:id", handler.UpdateRule)
	admin.DELETE("/rules/:id", handler.DeleteRule)
	admin.PATCH("/rules/:id/toggle", handler.ToggleRule)
	admin.GET("/stats/:rule_id", handler.GetRuleStats)

	agentGroup := apiGroup.Group("/agent")
	agentGroup.POST("/register", handler.AgentRegister)
	protectedAgent := agentGroup.Group("/")
	protectedAgent.Use(handler.RequireMachineToken())
	protectedAgent.POST("/heartbeat", handler.AgentHeartbeat)
	protectedAgent.POST("/stats", handler.AgentStats)
	protectedAgent.GET("/config", handler.AgentConfig)

	staticFS, err := fs.Sub(webFS, "web/dist")
	if err != nil {
		log.Fatalf("load embedded web assets failed: %v", err)
	}
	registerStaticRoutes(router, staticFS)

	ctx, stop := signalContext()
	defer stop()
	go runOfflineDetector(ctx, machineService)

	server := &http.Server{
		Addr:              ":" + strconv.Itoa(port),
		Handler:           router,
		ReadHeaderTimeout: requestTimeout,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	log.Printf("[INFO] panel listening on :%d", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("listen failed: %v", err)
	}
}

func registerStaticRoutes(router *gin.Engine, staticFS fs.FS) {
	indexBytes, err := fs.ReadFile(staticFS, "index.html")
	if err != nil {
		indexBytes = []byte("<html><body><h1>OnlyTun Panel</h1></body></html>")
	}

	router.StaticFS("/", http.FS(staticFS))

	router.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexBytes)
	})
}

func runOfflineDetector(ctx context.Context, machines *service.MachineService) {
	ticker := time.NewTicker(offlineCheckEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := machines.MarkOfflineBefore(time.Now().Add(-offlineAfter)); err != nil {
				log.Printf("[WARN] offline detector failed: %v", err)
			}
		}
	}
}

func signalContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan os.Signal, 1)
	signalNotify(ch)
	go func() {
		defer signalStop(ch)
		select {
		case <-ch:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
}

func envInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func signalNotify(ch chan<- os.Signal) {
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
}

func signalStop(ch chan<- os.Signal) {
	signal.Stop(ch)
}
