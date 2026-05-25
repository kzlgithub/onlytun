package api

import (
	"crypto/sha256"
	"errors"
	"net/http"
	"strings"
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
	Machines      *service.MachineService
	Rules         *service.RuleService
	Stats         *service.StatsService
	AdminPassword string
	JWTSecret     []byte
}

type loginRequest struct {
	Password string `json:"password"`
}

type loginResponse struct {
	Token string `json:"token"`
}

type adminClaims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

func NewHandler(machines *service.MachineService, rules *service.RuleService, stats *service.StatsService, adminPassword string) *Handler {
	sum := sha256.Sum256([]byte(adminPassword))
	return &Handler{
		Machines:      machines,
		Rules:         rules,
		Stats:         stats,
		AdminPassword: adminPassword,
		JWTSecret:     sum[:],
	}
}

func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if req.Password != h.AdminPassword {
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
	signed, err := token.SignedString(h.JWTSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to sign token"})
		return
	}

	c.JSON(http.StatusOK, loginResponse{Token: signed})
}

func (h *Handler) RequireAdminJWT() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, err := bearerToken(c.GetHeader("Authorization"))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}

		token, err := jwt.ParseWithClaims(tokenString, &adminClaims{}, func(token *jwt.Token) (any, error) {
			return h.JWTSecret, nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		c.Next()
	}
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
