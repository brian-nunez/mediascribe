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
	targetCode := strings.ToLower(strings.TrimSpace(targetLanguage))
	targetName := languageNameFromCode(targetCode)
	sourceName := "English"
	sourceCode := "en"
	if targetCode == "" {
		targetCode = "es"
		targetName = languageNameFromCode(targetCode)
	}

	prompt := fmt.Sprintf(`You are a professional %s (%s) to %s (%s) translator. Your goal is to accurately convey the meaning and nuances of the original %s text while adhering to %s grammar, vocabulary, and cultural sensitivities.
Produce only the %s translation, without any additional explanations or commentary. Please translate the following %s text into %s:


%s`, sourceName, sourceCode, targetName, targetCode, sourceName, targetName, targetName, sourceName, targetName, markdown)

	out, err := o.Client.Generate(ctx, o.Model, prompt)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func languageNameFromCode(code string) string {
	switch code {
	case "es":
		return "Spanish"
	case "fr":
		return "French"
	case "de":
		return "German"
	case "it":
		return "Italian"
	case "pt":
		return "Portuguese"
	case "ja":
		return "Japanese"
	case "ko":
		return "Korean"
	case "zh":
		return "Chinese"
	case "ru":
		return "Russian"
	case "ar":
		return "Arabic"
	case "hi":
		return "Hindi"
	case "en":
		return "English"
	default:
		// Fallback: use raw code as a label if unknown.
		return strings.ToUpper(code)
	}
}
