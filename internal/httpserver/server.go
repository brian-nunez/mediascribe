package httpserver

import (
	"context"
	"net/http"

	v1 "github.com/brian-nunez/video-to-blog-page/internal/handlers/v1"
	"github.com/brian-nunez/video-to-blog-page/internal/jobs"
)

type Server interface {
	Start(addr string) error
	Shutdown(ctx context.Context) error
}

type BootstrapConfig struct {
	StaticDirectories map[string]string
	APIHandler        http.Handler
	Jobs              *jobs.Service
	Observability     ObservabilityConfig
}

func Bootstrap(config BootstrapConfig) Server {
	server := New().
		WithStaticAssets(config.StaticDirectories).
		WithDefaultMiddleware(config.Observability).
		WithErrorHandler().
		WithRoutes(v1.RegisterRoutes(v1.RoutesConfig{APIHandler: config.APIHandler, Jobs: config.Jobs})).
		WithNotFound().
		Build()

	return server
}
