package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/brian-nunez/video-to-blog-page/internal/db"
	"github.com/brian-nunez/video-to-blog-page/internal/jobs"
)

type Handler struct {
	Jobs      *jobs.Service
	UIRootDir string
}

func (h Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/jobs", h.handleJobs)
	mux.HandleFunc("/api/jobs/", h.handleJobSubroutes)

	uiFS := http.FileServer(http.Dir(h.UIRootDir))
	mux.Handle("/", uiFS)
	return mux
}

func (h Handler) handleJobs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
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
		blog, filename, err := h.Jobs.GetBlogMarkdown(r.Context(), jobID)
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
			"download_path": fmt.Sprintf("/artifacts/jobs/%s/%s", jobID, filename),
		})
	case "retry":
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
