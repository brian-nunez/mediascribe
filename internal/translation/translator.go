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
	targetCode := strings.TrimSpace(targetLanguage)
	targetCodeLower := strings.ToLower(targetCode)
	targetName := languageNameFromCode(targetCode)
	sourceName := "English"
	sourceCode := "en"
	if targetCode == "" {
		targetCode = "es"
		targetCodeLower = "es"
		targetName = languageNameFromCode(targetCodeLower)
	}

	prompt := fmt.Sprintf(`You are a professional %s (%s) to %s (%s) translator. Your goal is to accurately convey the meaning and nuances of the original %s text while adhering to %s grammar, vocabulary, and cultural sensitivities.
Produce only the %s translation, without any additional explanations or commentary. Please translate the following %s text into %s:


%s`, sourceName, sourceCode, targetName, targetCodeLower, sourceName, targetName, targetName, sourceName, targetName, markdown)

	out, err := o.Client.Generate(ctx, o.Model, prompt)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}
