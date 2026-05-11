package markdown

import (
	"context"
	"fmt"
	"strings"

	"github.com/brian-nunez/video-to-blog-page/internal/ollama"
)

type Writer struct {
	Client *ollama.Client
	Model  string
}

func (w Writer) CreateOutline(ctx context.Context, transcript, analysis string) (string, error) {
	prompt := fmt.Sprintf(`Create a technical blog outline in Markdown.

Constraints:
- Keep it practical and implementation-focused.
- Include sections, subsections, and a short takeaway section.
- Avoid hype and generic filler.

Transcript:\n%s

Analysis:\n%s
`, transcript, analysis)
	resp, err := w.Client.Generate(ctx, w.Model, prompt)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp), nil
}

func (w Writer) CreateDraft(ctx context.Context, outline, transcript string) (string, error) {
	prompt := fmt.Sprintf(`Write a high-quality technical blog draft in Markdown.

Requirements:
- Follow this outline exactly.
- Include code-style snippets only when useful.
- Be precise, not fluffy.

Outline:\n%s

Transcript:\n%s
`, outline, transcript)
	resp, err := w.Client.Generate(ctx, w.Model, prompt)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp), nil
}

func (w Writer) RefineMarkdown(ctx context.Context, draft string) (string, error) {
	prompt := fmt.Sprintf(`Refine the following technical Markdown draft.

Requirements:
- Preserve technical accuracy.
- Improve flow and clarity.
- Keep headings and code/commands intact.
- Return Markdown only.

Draft:\n%s`, draft)
	resp, err := w.Client.Generate(ctx, w.Model, prompt)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp), nil
}
