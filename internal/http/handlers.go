package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/brian-nunez/video-to-blog-page/internal/auth"
	"github.com/brian-nunez/video-to-blog-page/internal/db"
	"github.com/brian-nunez/video-to-blog-page/internal/jobs"
	"github.com/brian-nunez/video-to-blog-page/internal/translation"
)

type Handler struct {
	Jobs *jobs.Service
	Auth auth.Service
}

func (h Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/public/catalog", h.handlePublicCatalog)
	mux.HandleFunc("/api/public/feed", h.handlePublicFeed)
	mux.HandleFunc("/api/public/blogs/", h.handlePublicBlogByID)
	mux.HandleFunc("/api/admin/login", h.handleAdminLogin)
	mux.HandleFunc("/api/admin/logout", h.handleAdminLogout)
	mux.HandleFunc("/api/admin/me", h.handleAdminMe)
	mux.HandleFunc("/api/admin/sections", h.handleAdminSections)
	mux.HandleFunc("/api/admin/sections/", h.handleAdminSectionByID)
	mux.HandleFunc("/api/admin/catalog", h.handleAdminCatalog)
	mux.HandleFunc("/api/admin/stats", h.handleAdminStats)
	mux.HandleFunc("/api/admin/blogs/", h.handleAdminBlogSubroutes)
	mux.HandleFunc("/api/admin/embeddings/rebuild", h.handleAdminEmbeddingsRebuild)
	mux.HandleFunc("/api/admin/artifacts/sync", h.handleAdminArtifactSync)
	mux.HandleFunc("/api/admin/batches", h.handleAdminBatches)
	mux.HandleFunc("/api/admin/batches/", h.handleAdminBatchByID)
	mux.HandleFunc("/api/admin/channel/videos", h.handleAdminChannelVideos)

	mux.HandleFunc("/api/search", h.handleSearch)
	mux.HandleFunc("/api/locales", h.handleLocales)
	mux.HandleFunc("/api/jobs", h.handleJobs)
	mux.HandleFunc("/api/jobs/", h.handleJobSubroutes)
	mux.HandleFunc("/", h.handleAPINotFound)
	return mux
}

func (h Handler) handleLocales(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"locales": translation.SupportedLocales(),
	})
}

func (h Handler) handleAPINotFound(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
}

func (h Handler) handlePublicCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	catalog, err := h.Jobs.ListPublicCatalog(r.Context())
	if err != nil {
		handleErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, catalog)
}

func (h Handler) handleAdminEmbeddingsRebuild(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{
			"status": h.Jobs.GetEmbeddingRebuildStatus(),
		})
	case http.MethodPost:
		st, err := h.Jobs.StartRebuildAllEmbeddings()
		if err != nil {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{"status": st})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h Handler) handleAdminArtifactSync(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	out, err := h.Jobs.SyncArtifactMetadata(r.Context())
	if err != nil {
		handleErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"result": out})
}

func (h Handler) handleAdminBatches(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		items, err := h.Jobs.ListBatches(r.Context())
		if err != nil {
			handleErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"batches": items})
	case http.MethodPost:
		var req jobs.CreateBatchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
			return
		}
		req.AutoStart = true
		out, err := h.Jobs.CreateBatch(r.Context(), req)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, out)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h Handler) handleAdminBatchByID(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	suffix := strings.TrimPrefix(r.URL.Path, "/api/admin/batches/")
	parts := strings.Split(strings.Trim(suffix, "/"), "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	batchID := parts[0]
	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		out, err := h.Jobs.GetBatch(r.Context(), batchID)
		if err != nil {
			handleErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
		return
	}

	if parts[1] != "start" || r.Method != http.MethodPost {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if err := h.Jobs.StartBatch(r.Context(), batchID); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h Handler) handleAdminChannelVideos(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		URL   string `json:"url"`
		Limit int    `json:"limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	items, err := h.Jobs.ListChannelVideos(r.Context(), req.URL, req.Limit)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handler) handlePublicFeed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	q := r.URL.Query()
	sectionID := strings.TrimSpace(q.Get("section_id"))
	language := strings.TrimSpace(q.Get("lang"))
	limit := 20
	offset := 0
	if v := strings.TrimSpace(q.Get("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	if v := strings.TrimSpace(q.Get("offset")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			offset = n
		}
	}
	page, err := h.Jobs.ListPublicFeedPage(r.Context(), sectionID, language, limit, offset)
	if err != nil {
		handleErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (h Handler) handlePublicBlogByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	blogID := strings.TrimPrefix(r.URL.Path, "/api/public/blogs/")
	blogID = strings.TrimSpace(blogID)
	if blogID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing blog id"})
		return
	}
	item, err := h.Jobs.GetPublicBlogByID(r.Context(), blogID)
	if err != nil {
		handleErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"blog": item})
}

func (h Handler) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	token, user, err := h.Auth.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
			return
		}
		handleErr(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     h.Auth.CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Now().Add(h.Auth.SessionTTL),
	})
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (h Handler) handleAdminLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	cookie, _ := r.Cookie(h.Auth.CookieName)
	if cookie != nil {
		_ = h.Auth.Logout(r.Context(), cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     h.Auth.CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h Handler) handleAdminMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	user, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (h Handler) handleAdminSections(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		items, err := h.Jobs.ListSections(r.Context())
		if err != nil {
			handleErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"sections": items})
	case http.MethodPost:
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
			return
		}
		item, err := h.Jobs.CreateSection(r.Context(), req.Name)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"section": item})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h Handler) handleAdminSectionByID(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	sectionID := strings.TrimPrefix(r.URL.Path, "/api/admin/sections/")
	sectionID = strings.TrimSpace(sectionID)
	if sectionID == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}

	switch r.Method {
	case http.MethodPut:
		var req struct {
			Name      string `json:"name"`
			SortOrder int    `json:"sort_order"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
			return
		}
		if err := h.Jobs.UpdateSection(r.Context(), sectionID, req.Name, req.SortOrder); err != nil {
			handleErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	case http.MethodDelete:
		if err := h.Jobs.DeleteSection(r.Context(), sectionID); err != nil {
			handleErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h Handler) handleAdminCatalog(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	catalog, allBlogs, err := h.Jobs.ListAdminCatalog(r.Context())
	if err != nil {
		handleErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"sections":    catalog.Sections,
		"unsectioned": catalog.Unsectioned,
		"blogs":       allBlogs,
	})
}

func (h Handler) handleAdminStats(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	stats, err := h.Jobs.AdminStats(r.Context())
	if err != nil {
		handleErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"stats": stats})
}

func (h Handler) handleAdminBlogSubroutes(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	suffix := strings.TrimPrefix(r.URL.Path, "/api/admin/blogs/")
	parts := strings.Split(strings.Trim(suffix, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	blogID := parts[0]
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			blog, err := h.Jobs.GetBlogByCatalogID(r.Context(), blogID, true)
			if err != nil {
				handleErr(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"blog": blog})
		case http.MethodPut:
			var req struct {
				Title     string `json:"title"`
				SectionID string `json:"section_id"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
				return
			}
			if err := h.Jobs.UpdateBlogMetadata(r.Context(), blogID, req.Title, req.SectionID); err != nil {
				handleErr(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		case http.MethodDelete:
			if err := h.Jobs.DeleteBlog(r.Context(), blogID); err != nil {
				handleErr(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		}
		return
	}

	switch parts[1] {
	case "restore":
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		if err := h.Jobs.RestoreBlog(r.Context(), blogID); err != nil {
			handleErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	case "section":
		if r.Method != http.MethodPut {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		var req struct {
			SectionID string `json:"section_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
			return
		}
		if err := h.Jobs.MoveBlogToSection(r.Context(), blogID, req.SectionID); err != nil {
			handleErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	case "content":
		if r.Method != http.MethodPut {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		var req struct {
			Language string `json:"language"`
			Markdown string `json:"markdown"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
			return
		}
		if err := h.Jobs.UpdateBlogLanguageContent(r.Context(), blogID, req.Language, req.Markdown); err != nil {
			handleErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	case "publish":
		if r.Method != http.MethodPut {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		var req struct {
			Published bool `json:"published"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
			return
		}
		if err := h.Jobs.SetBlogPublished(r.Context(), blogID, req.Published); err != nil {
			handleErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

func (h Handler) requireAdmin(w http.ResponseWriter, r *http.Request) (db.AdminUser, bool) {
	cookie, err := r.Cookie(h.Auth.CookieName)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return db.AdminUser{}, false
	}
	user, err := h.Auth.RequireUser(r.Context(), cookie.Value)
	if err != nil {
		if errors.Is(err, auth.ErrUnauthorized) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return db.AdminUser{}, false
		}
		handleErr(w, err)
		return db.AdminUser{}, false
	}
	return user, true
}

func (h Handler) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	limit := 10
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "limit must be a number"})
			return
		}
		limit = parsed
	}

	results, err := h.Jobs.SearchPublicBlogs(r.Context(), query, limit)
	if err != nil {
		handleErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"query":   query,
		"count":   len(results),
		"results": results,
	})
}

func (h Handler) handleJobs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		if _, ok := h.requireAdmin(w, r); !ok {
			return
		}
		var req jobs.CreateJobRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
			return
		}
		job, err := h.Jobs.CreateJob(r.Context(), req)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"job_id": job.ID})
	case http.MethodGet:
		items, err := h.Jobs.ListJobs(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, items)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h Handler) handleJobSubroutes(w http.ResponseWriter, r *http.Request) {
	suffix := strings.TrimPrefix(r.URL.Path, "/api/jobs/")
	parts := strings.Split(strings.Trim(suffix, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	jobID := parts[0]

	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		job, err := h.Jobs.GetJob(r.Context(), jobID)
		if err != nil {
			handleErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, job)
		return
	}

	switch parts[1] {
	case "transcript":
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		payload, err := h.Jobs.GetTranscript(r.Context(), jobID)
		if err != nil {
			if errors.Is(err, jobs.ErrNotReady) {
				job, jobErr := h.Jobs.GetJob(r.Context(), jobID)
				if jobErr != nil {
					handleErr(w, jobErr)
					return
				}
				writeJSON(w, http.StatusOK, map[string]any{
					"job_id":        jobID,
					"ready":         false,
					"status":        job.Status,
					"current_stage": job.CurrentStage,
					"transcript":    "",
				})
				return
			}
			handleErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"job_id":     jobID,
			"ready":      true,
			"transcript": payload,
		})
	case "blog":
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		lang := strings.TrimSpace(r.URL.Query().Get("lang"))
		blog, filename, err := h.Jobs.GetBlogMarkdown(r.Context(), jobID, lang)
		if err != nil {
			if errors.Is(err, jobs.ErrNotReady) {
				job, jobErr := h.Jobs.GetJob(r.Context(), jobID)
				if jobErr != nil {
					handleErr(w, jobErr)
					return
				}
				writeJSON(w, http.StatusOK, map[string]any{
					"job_id":        jobID,
					"ready":         false,
					"status":        job.Status,
					"current_stage": job.CurrentStage,
					"markdown":      "",
					"download_path": "",
				})
				return
			}
			handleErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"job_id":        jobID,
			"ready":         true,
			"markdown":      blog,
			"language":      lang,
			"download_path": fmt.Sprintf("/artifacts/jobs/%s/%s", jobID, filename),
		})
	case "translations":
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		items, err := h.Jobs.ListTranslations(r.Context(), jobID)
		if err != nil {
			handleErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"job_id":       jobID,
			"translations": items,
			"activity":     h.Jobs.GetTranslationActivity(jobID),
		})
	case "translate":
		if _, ok := h.requireAdmin(w, r); !ok {
			return
		}
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		var req struct {
			Language string `json:"language"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
			return
		}
		out, err := h.Jobs.StartTranslateCompletedBlog(jobID, req.Language)
		if err != nil {
			handleErr(w, err)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{
			"job_id":      jobID,
			"translation": out,
		})
	case "retry":
		if _, ok := h.requireAdmin(w, r); !ok {
			return
		}
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		if err := h.Jobs.RetryJob(r.Context(), jobID); err != nil {
			handleErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"job_id": jobID, "status": "queued"})
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func handleErr(w http.ResponseWriter, err error) {
	if errors.Is(err, db.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
}
