package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/slurmtack/slurmtack/internal/service"
	"github.com/slurmtack/slurmtack/internal/slurm"
	"github.com/slurmtack/slurmtack/internal/store"
	"github.com/slurmtack/slurmtack/internal/trace"
)

type ServerOption func(*serverOptions)

type serverOptions struct {
	jwtManager  *JWTManager
	slurmClient slurm.Client
}

func WithJWTAuth(jwtManager *JWTManager, slurmClient slurm.Client) ServerOption {
	return func(o *serverOptions) {
		o.jwtManager = jwtManager
		o.slurmClient = slurmClient
	}
}


type Server struct {
	httpServer *http.Server
	engine     *gin.Engine
}

func NewServer(listenAddr string, sqlStore *store.SQLiteStore, svc *service.SwitchService, inventoryHandler *InventoryHandler, logger *slog.Logger, opts ...ServerOption) *Server {
	var sopts serverOptions
	for _, o := range opts {
		o(&sopts)
	}

	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(accessLogMiddleware(trace.OrDefault(logger).With("component", "api")), gin.Recovery())

	healthHandler := NewHealthHandler(sqlStore)
	engine.GET("/health", healthHandler.Check)

	if sopts.jwtManager != nil && sopts.slurmClient != nil {
		authHandler := NewAuthHandler(sopts.jwtManager, sopts.slurmClient)
		engine.POST("/v1/auth/login", authHandler.Login)
	}

	switchHandler := NewSwitchHandler(svc, sqlStore)
	v1 := engine.Group("/v1", BearerAuth(sopts.jwtManager))
	{
		v1.POST("/switches", switchHandler.Create)
		v1.GET("/switches/:id", switchHandler.Get)
		v1.GET("/switches/:id/steps", switchHandler.Steps)
		v1.GET("/switches", switchHandler.List)
		v1.POST("/switches/:id/cancel", switchHandler.Cancel)
		if inventoryHandler != nil {
			v1.GET("/dashboard/inventory", inventoryHandler.Get)
		}
	}

	return &Server{
		engine: engine,
		httpServer: &http.Server{
			Addr:    listenAddr,
			Handler: engine,
		},
	}
}

func accessLogMiddleware(logger *slog.Logger) gin.HandlerFunc {
	logger = trace.OrDefault(logger)

	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		route := c.FullPath()
		if route == "" {
			route = c.Request.URL.Path
		}

		logger.Info("api.request",
			"event", "api.request",
			"method", c.Request.Method,
			"route", route,
			"path", c.Request.URL.Path,
			"status_code", c.Writer.Status(),
			"latency", time.Since(start),
			"client_ip", c.ClientIP(),
		)
	}
}

func (s *Server) Start() error {
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) Engine() *gin.Engine {
	return s.engine
}
