package planner

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/brian-nunez/video-to-blog-page/internal/ollama"
)

type Decision struct {
	NextStage       string   `json:"next_stage"`
	Reason          string   `json:"reason"`
	RequiredInputs  []string `json:"required_inputs"`
	ExpectedOutputs []string `json:"expected_outputs"`
	SafeToRun       bool     `json:"safe_to_run"`
}

type Planner struct {
	Client *ollama.Client
	Model  string
}

func (p Planner) Decide(ctx context.Context, stage string, requiredInputs, expectedOutputs []string) (Decision, error) {
	prompt := fmt.Sprintf(`You are a strict pipeline planner.

Return JSON only with keys:
- next_stage
- reason
- required_inputs
- expected_outputs
- safe_to_run

Evaluate if stage %q can run.
Required inputs: %v
Expected outputs: %v
`, stage, requiredInputs, expectedOutputs)

	raw, err := p.Client.Generate(ctx, p.Model, prompt)
	if err != nil {
		return Decision{}, err
	}
	var decision Decision
	if err := json.Unmarshal([]byte(raw), &decision); err != nil {
		return Decision{}, fmt.Errorf("planner JSON parse error: %w", err)
	}
	return decision, nil
}
