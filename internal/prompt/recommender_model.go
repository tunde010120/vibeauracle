package prompt

import (
	"context"
	"fmt"
	"strings"
	"encoding/json"
)

// ModelRecommender uses an AI model to generate background recommendations.
type ModelRecommender struct {
	model Model
}

func NewModelRecommender(m Model) *ModelRecommender {
	return &ModelRecommender{model: m}
}

func (r *ModelRecommender) Recommend(ctx context.Context, in RecommendInput) ([]Recommendation, error) {
	if r.model == nil {
		return nil, nil
	}

	// Craft a very concise system prompt for the background recommendation task.
	// We use a high "Modular Intent" instruction to keep it focused.
	backgroundPrompt := fmt.Sprintf(`You are a background codebase recommender.
The user just sent this prompt: "%s" (Intent: %s)
In the directory: %s

Based on this, suggest 1-2 highly relevant, granular next steps or "recommended actions".
Output MUST be a JSON array of objects with "title", "description", and "confidence" (0-1).
Keep descriptions under 15 words.
Example: [{"title": "Add Unit Tests", "description": "Add tests for the new auth handler logic.", "confidence": 0.9}]`, 
		in.UserText, in.Intent, in.WorkingDir)

	resp, err := r.model.Generate(ctx, backgroundPrompt)
	if err != nil {
		return nil, fmt.Errorf("recommender model call: %w", err)
	}

	// Try to extract JSON from markdown if some models wrap it.
	jsonStr := resp
	if start := strings.Index(resp, "["); start != -1 {
		if end := strings.LastIndex(resp, "]"); end != -1 && end > start {
			jsonStr = resp[start : end+1]
		}
	}

	var recs []Recommendation
	if err := json.Unmarshal([]byte(jsonStr), &recs); err != nil {
		return nil, fmt.Errorf("parsing recommendations: %w (raw response: %s)", err, resp)
	}

	return recs, nil
}
