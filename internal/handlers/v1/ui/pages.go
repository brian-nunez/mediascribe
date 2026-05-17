package uihandlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/brian-nunez/video-to-blog-page/internal/db"
	"github.com/brian-nunez/video-to-blog-page/internal/jobs"
	"github.com/brian-nunez/video-to-blog-page/internal/observability"
	"github.com/brian-nunez/video-to-blog-page/views/pages"
	"github.com/labstack/echo/v4"
)

type Handlers struct {
	Jobs *jobs.Service
}

func New(jobService *jobs.Service) Handlers {
	return Handlers{Jobs: jobService}
}

func render(c echo.Context, component templ.Component, message string) error {
	observability.LogInfo(c.Request().Context(), message)
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
	return component.Render(c.Request().Context(), c.Response().Writer)
}

func (h Handlers) HomeHandler(c echo.Context) error {
	data := pages.FeedViewData{
		SelectedSection:  selectedSection(c.QueryParam("section_id")),
		SelectedLanguage: selectedLanguage(c.QueryParam("lang")),
	}
	if h.Jobs == nil {
		data.LoadError = "Feed service is unavailable."
		return render(c, pages.Home(data), "home_page_loaded")
	}

	limit := queryInt(c, "limit", 20)
	offset := queryInt(c, "offset", 0)
	page, err := h.Jobs.ListPublicFeedPage(c.Request().Context(), data.SelectedSection, data.SelectedLanguage, limit, offset)
	if err != nil {
		data.LoadError = err.Error()
		return render(c, pages.Home(data), "home_page_loaded")
	}
	data.Page = page
	if h.Jobs != nil {
		if msg, err := h.Jobs.GetGlobalSetting(c.Request().Context(), "dashboard_message"); err == nil {
			data.GlobalMessage = msg
		}
		if nType, err := h.Jobs.GetGlobalSetting(c.Request().Context(), "notice_type"); err == nil {
			data.NoticeType = nType
		}
		if bText, err := h.Jobs.GetGlobalSetting(c.Request().Context(), "banner_text"); err == nil {
			data.BannerText = bText
		}
		if bEnabled, err := h.Jobs.GetGlobalSetting(c.Request().Context(), "banner_enabled"); err == nil {
			data.BannerEnabled = bEnabled == "true"
		}
	}
	return render(c, pages.Home(data), "home_page_loaded")
}

func (h Handlers) BlogHandler(c echo.Context) error {
	blogID := strings.TrimSpace(c.Param("id"))
	if blogID == "" {
		blogID = strings.TrimSpace(c.QueryParam("id"))
	}
	data := pages.BlogViewData{
		SelectedLanguage: c.QueryParam("lang"),
		SearchQuery:      c.QueryParam("q"),
		Chunk:            c.QueryParam("chunk"),
	}
	if blogID == "" {
		c.Response().Status = http.StatusBadRequest
		data.LoadError = "Missing blog id in URL."
		return render(c, pages.Blog(data), "blog_page_loaded")
	}
	if h.Jobs == nil {
		c.Response().Status = http.StatusServiceUnavailable
		data.LoadError = "Blog service is unavailable."
		return render(c, pages.Blog(data), "blog_page_loaded")
	}

	blog, err := h.Jobs.GetPublicBlogByID(c.Request().Context(), blogID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.Response().Status = http.StatusNotFound
			return render(c, pages.Blog(data), "blog_page_loaded")
		}
		c.Response().Status = http.StatusInternalServerError
		data.LoadError = err.Error()
		return render(c, pages.Blog(data), "blog_page_loaded")
	}
	data.Blog = blog
	data.Found = true
	return render(c, pages.Blog(data), "blog_page_loaded")
}

func (h Handlers) AdminDashboardHandler(c echo.Context) error {
	return render(c, pages.AdminDashboard(), "admin_dashboard_loaded")
}

func (h Handlers) AdminLoginHandler(c echo.Context) error {
	return render(c, pages.AdminLogin(), "admin_login_loaded")
}

func (h Handlers) AdminSectionsHandler(c echo.Context) error {
	return render(c, pages.AdminSections(), "admin_sections_loaded")
}

func (h Handlers) AdminBlogsHandler(c echo.Context) error {
	return render(c, pages.AdminBlogs(), "admin_blogs_loaded")
}

func (h Handlers) AdminBlogEditHandler(c echo.Context) error {
	return render(c, pages.AdminBlogEdit(), "admin_blog_edit_loaded")
}

func (h Handlers) AdminBatchesHandler(c echo.Context) error {
	return render(c, pages.AdminBatches(), "admin_batches_loaded")
}

func selectedSection(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "all"
	}
	return value
}

func selectedLanguage(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "all"
	}
	return value
}

func queryInt(c echo.Context, name string, fallback int) int {
	value := strings.TrimSpace(c.QueryParam(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
