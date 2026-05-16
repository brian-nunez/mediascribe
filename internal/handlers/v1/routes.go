package v1

import (
	"net/http"

	uihandlers "github.com/brian-nunez/video-to-blog-page/internal/handlers/v1/ui"
	"github.com/brian-nunez/video-to-blog-page/internal/jobs"
	"github.com/labstack/echo/v4"
)

type RoutesConfig struct {
	APIHandler http.Handler
	Jobs       *jobs.Service
}

func RegisterRoutes(config RoutesConfig) func(e *echo.Echo) {
	return func(e *echo.Echo) {
		ui := uihandlers.New(config.Jobs)

		e.GET("/", ui.HomeHandler)
		e.GET("/index.html", ui.HomeHandler)
		e.GET("/blog", ui.BlogHandler)
		e.GET("/blog/", ui.BlogHandler)
		e.GET("/blog.html", ui.BlogHandler)
		e.GET("/blog/:id", ui.BlogHandler)

		e.GET("/admin", ui.AdminDashboardHandler)
		e.GET("/admin/", ui.AdminDashboardHandler)
		e.GET("/admin/login", ui.AdminLoginHandler)
		e.GET("/admin/login/", ui.AdminLoginHandler)
		e.GET("/admin/sections", ui.AdminSectionsHandler)
		e.GET("/admin/sections/", ui.AdminSectionsHandler)
		e.GET("/admin/blogs", ui.AdminBlogsHandler)
		e.GET("/admin/blogs/", ui.AdminBlogsHandler)
		e.GET("/admin/blogs/:id", ui.AdminBlogEditHandler)
		e.GET("/admin/batches", ui.AdminBatchesHandler)
		e.GET("/admin/batches/", ui.AdminBatchesHandler)

		e.GET("/manifest.webmanifest", func(c echo.Context) error {
			c.Response().Header().Set(echo.HeaderContentType, "application/manifest+json")
			return c.File("assets/manifest.webmanifest")
		})
		e.GET("/sw.js", func(c echo.Context) error { return c.File("assets/sw.js") })
		e.GET("/api/v1/health", HealthHandler)

		if config.APIHandler != nil {
			wrapped := echo.WrapHandler(config.APIHandler)
			e.Any("/api", wrapped)
			e.Any("/api/*", wrapped)
		}
	}
}
