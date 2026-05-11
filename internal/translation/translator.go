package translation

import (
	"context"
	"fmt"
	"strings"

	"github.com/brian-nunez/video-to-blog-page/internal/ollama"
)

type Translator interface {
	TranslateMarkdown(ctx context.Context, markdown, targetLanguage string) (string, error)
}

type OllamaTranslator struct {
	Client *ollama.Client
	Model  string
}

func (o OllamaTranslator) TranslateMarkdown(ctx context.Context, markdown, targetLanguage string) (string, error) {
	prompt := fmt.Sprintf(`Translate the following Markdown from English to %s.

Rules:
- Preserve Markdown formatting exactly.
- Preserve all code blocks exactly.
- Preserve commands, filenames, paths, URLs, package names, ports, and IP addresses.
- Only translate human-readable prose.
- Return only translated Markdown with no commentary.

Markdown:\n\n%s`, targetLanguage, markdown)

	out, err := o.Client.Generate(ctx, o.Model, prompt)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}
