package httpapi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func New(db *gorm.DB, logger *zap.Logger, corsOrigins []string, jwtSecrets ...string) http.Handler {
	secret := "development-only-change-this-secret-before-deploying"
	if len(jwtSecrets) > 0 && jwtSecrets[0] != "" {
		secret = jwtSecrets[0]
	}
	router := gin.New()
	router.Use(gin.Recovery(), requestLogger(logger), cors(corsOrigins))
	router.GET("/healthz", func(c *gin.Context) {
		sqlDB, err := db.DB()
		if err == nil {
			err = sqlDB.PingContext(c.Request.Context())
		}
		if err != nil {
			fail(c, http.StatusServiceUnavailable, "database_unavailable", "database health check failed")
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	api := router.Group("/api/v1")
	auth := api.Group("/auth")
	auth.POST("/register", register(db, secret))
	auth.POST("/login", login(db, secret))
	protected := api.Group("")
	protected.Use(requireAuth(secret))
	protected.GET("/auth/me", currentUser(db))
	protected.GET("/documents", listDocuments(db))
	protected.POST("/documents", createDocument(db))
	protected.GET("/documents/:id", getDocument(db))
	protected.PUT("/documents/:id", updateDocument(db))
	protected.DELETE("/documents/:id", deleteDocument(db))
	return router
}

func requestLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()
		logger.Info("http request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("duration", time.Since(started)),
			zap.String("client_ip", c.ClientIP()),
		)
	}
}

func cors(allowed []string) gin.HandlerFunc {
	allow := make(map[string]struct{}, len(allowed))
	for _, origin := range allowed {
		allow[origin] = struct{}{}
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if _, ok := allow[origin]; ok && origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, If-Match")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
