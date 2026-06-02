package api

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	paneldb "github.com/onlytun/panel/db"
	"github.com/onlytun/panel/service"
)

const (
	ContextMachineKey = "machine"
	jwtTTL            = 24 * time.Hour
)

type Handler struct {
	Machines          *service.MachineService
	Rules             *service.RuleService
	Groups            *service.GroupService
	Stats             *service.StatsService
	Settings          *service.SettingsService
	mu                sync.RWMutex
	AdminPasswordHash string
	JWTSecret         []byte
}

type loginRequest struct {
	Password string `json:"password"`
}

type loginResponse struct {
	Token string `json:"token"`
}

type changePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

type adminClaims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

func NewHandler(machines *service.MachineService, rules *service.RuleService, groups *service.GroupService, stats *service.StatsService, settings *service.SettingsService, adminPasswordHash string) *Handler {
	return &Handler{
		Machines:          machines,
		Rules:             rules,
		Groups:            groups,
		Stats:             stats,
		Settings:          settings,
		AdminPasswordHash: adminPasswordHash,
		JWTSecret:         []byte(adminPasswordHash),
	}
}

func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if !h.passwordMatches(req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid password"})
		return
	}

	claims := adminClaims{
		Role: "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "admin",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(jwtTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(h.jwtSecret())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to sign token"})
		return
	}

	c.JSON(http.StatusOK, loginResponse{Token: signed})
}

func (h *Handler) ChangePassword(c *gin.Context) {
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if strings.TrimSpace(req.NewPassword) == "" || len(req.NewPassword) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "new password must be at least 8 characters"})
		return
	}
	if !h.passwordMatches(req.OldPassword) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "old password is incorrect"})
		return
	}

	nextHash := service.HashAdminPassword(req.NewPassword)
	if err := h.Settings.SetAdminPasswordHash(nextHash); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.mu.Lock()
	h.AdminPasswordHash = nextHash
	h.JWTSecret = []byte(nextHash)
	h.mu.Unlock()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) RequireAdminJWT() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, err := bearerToken(c.GetHeader("Authorization"))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}

		token, err := jwt.ParseWithClaims(tokenString, &adminClaims{}, func(token *jwt.Token) (any, error) {
			return h.jwtSecret(), nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		c.Next()
	}
}

func (h *Handler) passwordMatches(password string) bool {
	hash := service.HashAdminPassword(password)
	h.mu.RLock()
	current := h.AdminPasswordHash
	h.mu.RUnlock()
	return subtle.ConstantTimeCompare([]byte(hash), []byte(current)) == 1
}

func (h *Handler) jwtSecret() []byte {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return append([]byte(nil), h.JWTSecret...)
}

func (h *Handler) RequireMachineToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := bearerToken(c.GetHeader("Authorization"))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}

		machine, err := h.Machines.AuthenticateMachineToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid machine token"})
			return
		}

		c.Set(ContextMachineKey, machine)
		c.Next()
	}
}

func MachineFromContext(c *gin.Context) (*paneldb.Machine, bool) {
	value, ok := c.Get(ContextMachineKey)
	if !ok {
		return nil, false
	}
	machine, ok := value.(*paneldb.Machine)
	return machine, ok
}

func ClientIP(c *gin.Context) string {
	ip := strings.TrimSpace(c.ClientIP())
	if ip != "" {
		return ip
	}
	return c.Request.RemoteAddr
}

func bearerToken(header string) (string, error) {
	header = strings.TrimSpace(header)
	if header == "" {
		return "", errors.New("missing authorization header")
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", errors.New("invalid authorization header")
	}
	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", errors.New("empty token")
	}
	return token, nil
}
