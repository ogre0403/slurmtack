package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/slurmtack/slurmtack/internal/service"
	"github.com/slurmtack/slurmtack/internal/store"
)

type Server struct {
	httpServer *http.Server
	engine     *gin.Engine
}

func NewServer(listenAddr string, token string, sqlStore *store.SQLiteStore, svc *service.SwitchService) *Server {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())

	healthHandler := NewHealthHandler(sqlStore)
	engine.GET("/health", healthHandler.Check)

	switchHandler := NewSwitchHandler(svc, sqlStore)
	v1 := engine.Group("/v1", BearerAuth(token))
	{
		v1.POST("/switches", switchHandler.Create)
		v1.GET("/switches/:id", switchHandler.Get)
		v1.GET("/switches", switchHandler.List)
		v1.POST("/switches/:id/cancel", switchHandler.Cancel)
	}

	return &Server{
		engine: engine,
		httpServer: &http.Server{
			Addr:    listenAddr,
			Handler: engine,
		},
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
