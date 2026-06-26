package api

import (
	"branchscribe/backend/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewRouter(cfg config.Config, db *pgxpool.Pool) *gin.Engine {
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	_ = router.SetTrustedProxies(nil)
	router.Use(gin.Recovery())
	router.Use(RequestID())
	router.Use(CORS())

	router.GET("/health", healthHandler(db))

	api := router.Group("/api")
	api.GET("/health", healthHandler(db))

	return router
}
