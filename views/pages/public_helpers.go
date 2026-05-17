package pages

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/brian-nunez/video-to-blog-page/internal/jobs"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

type FeedViewData struct {
	Page             jobs.PublicFeedPage `json:"page"`
	SelectedSection  string              `json:"selected_section"`
	SelectedLanguage string              `json:"selected_language"`
	GlobalMessage    string              `json:"global_message,omitempty"`
	NoticeType       string              `json:"notice_type,omitempty"`
	BannerEnabled    bool                `json:"banner_enabled,omitempty"`
	BannerText       string              `json:"banner_text,omitempty"`
	LoadError        string              `json:"load_error,omitempty"`
}

type BlogViewData struct {
	Blog             jobs.BlogView `json:"blog"`
	SelectedLanguage string        `json:"selected_language"`
	SearchQuery      string        `json:"search_query,omitempty"`
	Chunk            string        `json:"chunk,omitempty"`
	Found            bool          `json:"found"`
	LoadError        string        `json:"load_error,omitempty"`
}

func blogPageTitle(data BlogViewData) string {
	if data.Found && strings.TrimSpace(data.Blog.Title) != "" {
		return data.Blog.Title + " | MediaScribe"
	}
	return "MediaScribe | Blog"
}

var markdownRenderer = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
)

func selectedSectionID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "all"
	}
	return value
}

func selectedLanguageID(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "all"
	}
	return value
}

func noticeClass(t string) string {
	switch t {
	case "warning":
		return "border-amber-200 bg-amber-50/50"
	case "error":
		return "border-red-200 bg-red-50/50"
	case "success":
		return "border-emerald-200 bg-emerald-50/50"
	default:
		return "border-blue-200 bg-blue-50/50"
	}
}

func noticeTextClass(t string) string {
	switch t {
	case "warning":
		return "text-amber-900"
	case "error":
		return "text-red-900"
	case "success":
		return "text-emerald-900"
	default:
		return "text-blue-900"
	}
}

func feedMeta(data FeedViewData) string {
	if data.LoadError != "" {
		return "Unable to load published blogs."
	}
	count := len(data.Page.Items)
	if data.Page.Total == 0 {
		return "No blogs for this filter."
	}
	label := "blogs"
	if data.Page.Total == 1 {
		label = "blog"
	}
	return fmt.Sprintf("%d of %d %s", count, data.Page.Total, label)
}

func sectionCount(data FeedViewData, sectionID string) int {
	if sectionID == "all" || sectionID == "" {
		return data.Page.Total
	}
	for _, section := range data.Page.Sections {
		if section.ID == sectionID {
			return section.Count
		}
	}
	return 0
}

func languageCountTotal(data FeedViewData) int {
	total := 0
	for _, count := range data.Page.LanguageCounts {
		total += count
	}
	return total
}

func sortedLanguageCounts(data FeedViewData) []struct {
	Language string
	Count    int
} {
	out := make([]struct {
		Language string
		Count    int
	}, 0, len(data.Page.LanguageCounts))
	for language, count := range data.Page.LanguageCounts {
		language = strings.TrimSpace(strings.ToLower(language))
		if language == "" || count <= 0 {
			continue
		}
		out = append(out, struct {
			Language string
			Count    int
		}{Language: language, Count: count})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Language < out[j].Language })
	return out
}

func dateLabel(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("Jan 2, 2006")
}

func sourceValue(sourceURL, sourcePath string) string {
	if strings.TrimSpace(sourceURL) != "" {
		return strings.TrimSpace(sourceURL)
	}
	return strings.TrimSpace(sourcePath)
}

func sourceDomain(sourceURL, sourcePath string) string {
	source := sourceValue(sourceURL, sourcePath)
	if source == "" {
		return "no source"
	}
	if parsed, err := url.Parse(source); err == nil && parsed.Host != "" {
		return strings.TrimPrefix(parsed.Host, "www.")
	}
	return "local path"
}

func shortJobID(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 8 {
		return value
	}
	return value[:8]
}

func isHTTPURL(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

func safeURL(value string) templ.SafeURL {
	return templ.URL(value)
}

func sectionLinkClass(active bool) string {
	if active {
		return "block w-full rounded-full bg-[var(--primary)] px-3 py-2 text-left text-sm text-white transition"
	}
	return "block w-full rounded-full px-3 py-2 text-left text-sm text-slate-700 transition hover:bg-slate-100"
}

func languageFilterClass(active bool) string {
	if active {
		return "block w-full rounded-full bg-slate-900 px-3 py-1.5 text-left text-xs text-white transition"
	}
	return "block w-full rounded-full px-3 py-1.5 text-left text-xs text-slate-700 transition hover:bg-slate-100"
}

func languageTabClass(active bool) string {
	if active {
		return "rounded-full border border-slate-900 bg-slate-900 px-2.5 py-1 text-xs text-white transition"
	}
	return "rounded-full border border-slate-300 bg-white px-2.5 py-1 text-xs text-slate-600 transition hover:bg-slate-100"
}

func blogSummaryURL(item jobs.PublicBlogSummary) string {
	return "/blog/" + url.PathEscape(item.ID) + "?lang=en"
}

func blogLanguageURL(blog jobs.BlogView, language string) string {
	return "/blog/" + url.PathEscape(blog.ID) + "?lang=" + url.QueryEscape(language)
}

func sectionURL(sectionID, language string) string {
	q := url.Values{}
	if strings.TrimSpace(sectionID) != "" && sectionID != "all" {
		q.Set("section_id", sectionID)
	}
	if strings.TrimSpace(language) != "" && language != "all" {
		q.Set("lang", language)
	}
	if encoded := q.Encode(); encoded != "" {
		return "/?" + encoded
	}
	return "/"
}

func languageURL(sectionID, language string) string {
	q := url.Values{}
	if strings.TrimSpace(sectionID) != "" && sectionID != "all" {
		q.Set("section_id", sectionID)
	}
	if strings.TrimSpace(language) != "" && language != "all" {
		q.Set("lang", language)
	}
	if encoded := q.Encode(); encoded != "" {
		return "/?" + encoded
	}
	return "/"
}

func selectedBlogLanguage(data BlogViewData) jobs.BlogLanguageContent {
	langs := data.Blog.Languages
	if len(langs) == 0 {
		return jobs.BlogLanguageContent{Language: jobs.OriginalLanguage}
	}
	selected := strings.TrimSpace(strings.ToLower(data.SelectedLanguage))
	if selected != "" {
		for _, item := range langs {
			if strings.EqualFold(item.Language, selected) {
				return item
			}
		}
	}
	for _, item := range langs {
		if strings.EqualFold(item.Language, jobs.OriginalLanguage) {
			return item
		}
	}
	return langs[0]
}

func matchMessage(data BlogViewData) string {
	if data.Chunk == "" {
		return ""
	}
	if data.SearchQuery != "" {
		return fmt.Sprintf("Opened from search match: chunk %s for %q", data.Chunk, data.SearchQuery)
	}
	return "Opened from search match: chunk " + data.Chunk
}

func markdownHTML(raw string) templ.Component {
	var buf bytes.Buffer
	if err := markdownRenderer.Convert([]byte(raw), &buf); err != nil {
		return templ.Raw("<p>Unable to render article markdown.</p>")
	}
	return templ.Raw(buf.String())
}

func jsonScript(id string, payload any) templ.Component {
	raw, err := json.Marshal(payload)
	if err != nil {
		raw = []byte("{}")
	}
	return templ.Raw(`<script id="` + id + `" type="application/json">` + string(raw) + `</script>`)
}
