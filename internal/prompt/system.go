package prompt

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nathfavour/vibeauracle/sys"
)

// System is the modular prompt engine: classify → layer instructions → build prompt → parse response.
type System struct {
	cfg         *sys.Config
	memory      Memory
	recommender Recommender

	// Budgeting to avoid unintended spend.
	recoUsed int
}

func New(cfg *sys.Config, memory Memory, recommender Recommender) *System {
	return &System{cfg: cfg, memory: memory, recommender: recommender}
}

// SetRecommender updates the active recommender.
func (s *System) SetRecommender(r Recommender) {
	s.recommender = r
}

// Build produces the prompt envelope for a user input.
func (s *System) Build(ctx context.Context, userText string, snapshot sys.Snapshot, toolDefs string) (Envelope, []Recommendation, error) {
	intent := ClassifyIntent(userText)
	if s.cfg != nil && s.cfg.Prompt.Mode != "" {
		// Config can force a mode. "auto" keeps classification.
		mode := strings.ToLower(strings.TrimSpace(s.cfg.Prompt.Mode))
		switch mode {
		case "auto":
			// keep
		case "ask":
			intent = IntentAsk
		case "plan":
			intent = IntentPlan
		case "crud":
			intent = IntentCRUD
		}
	}

	if !LooksLikePrompt(userText) {
		return Envelope{Intent: intent, Prompt: "", Instructions: nil, Metadata: map[string]any{"ignored": true}}, nil, nil
	}

	instructions := s.layers(intent)

	// Learning layer: cheap recall injection.
	var recall string
	if s.cfg != nil && s.cfg.Prompt.LearningEnabled && s.memory != nil {
		snips, _ := s.memory.Recall(userText)
		if len(snips) > 0 {
			recall = strings.Join(snips, "\n")
		}
	}

	prompt := s.compose(intent, instructions, recall, snapshot, toolDefs, userText)

	// Learning write-back: store a compact behavioral signal for future recall.
	if s.cfg != nil && s.cfg.Prompt.LearningEnabled && s.memory != nil {
		compact := userText
		if len(compact) > 160 {
			compact = compact[:160]
		}
		_ = s.memory.Store(fmt.Sprintf("prompt:%d", time.Now().UnixNano()), fmt.Sprintf("intent=%s text=%s", intent, compact))
	}

	recs, err := s.maybeRecommend(ctx, intent, userText, snapshot.WorkingDir)
	if err != nil {
		// Recommendations are best-effort and must never fail the main prompt.
		recs = nil
	}

	return Envelope{
		Intent:       intent,
		Prompt:       prompt,
		Instructions: instructions,
		Metadata: map[string]any{
			"working_dir": snapshot.WorkingDir,
			"cpu":         snapshot.CPUUsage,
			"mem":         snapshot.MemoryUsage,
		},
	}, recs, nil
}

func (s *System) layers(intent Intent) []string {
	layers := []string{}

	// Base system layer
	layers = append(layers, "You are vibeauracle's core assistant. Be accurate, safe, and helpful.")
	layers = append(layers, "If you are unsure, ask a clarifying question instead of guessing.")

	// Safety layer: reflect tool security model
	layers = append(layers, "Tools may require explicit permissions; never request sensitive data unless necessary.")

	// Project layer (configurable)
	if s.cfg != nil {
		if strings.TrimSpace(s.cfg.Prompt.ProjectInstructions) != "" {
			layers = append(layers, s.cfg.Prompt.ProjectInstructions)
		}
	}

	// Mode layer
	switch intent {
	case IntentAsk:
		layers = append(layers, "MODE=ASK. Answer clearly and concisely. Prefer explanation over action.")
	case IntentPlan:
		layers = append(layers, "MODE=PLAN. Provide a structured plan with checkpoints, risks, and next steps.")
	case IntentCRUD:
		layers = append(layers, "MODE=CRUD. Propose concrete file/code changes and describe them precisely.")
	default:
		layers = append(layers, "MODE=CHAT. Be conversational but efficient.")
	}

	return layers
}

func (s *System) compose(intent Intent, layers []string, recall string, snapshot sys.Snapshot, toolDefs string, userText string) string {
	b := strings.Builder{}
	b.WriteString("SYSTEM INSTRUCTIONS:\n")
	for _, l := range layers {
		b.WriteString("- ")
		b.WriteString(l)
		b.WriteString("\n")
	}

	if strings.TrimSpace(recall) != "" {
		b.WriteString("\nLEARNING/RECALL (local):\n")
		b.WriteString(recall)
		b.WriteString("\n")
	}

	b.WriteString("\nSYSTEM SNAPSHOT:\n")
	b.WriteString(fmt.Sprintf("CWD: %s\nCPU: %.2f%%\nMEM: %.2f%%\n", snapshot.WorkingDir, snapshot.CPUUsage, snapshot.MemoryUsage))

	if strings.TrimSpace(toolDefs) != "" {
		b.WriteString("\nAVAILABLE TOOLS (ACTION REQUIRED):\n")
		b.WriteString(toolDefs)
		b.WriteString("\nYou are an ACTION-FIRST agent. If the user request implies `creating`, `reading`, or `modifying` files, you MUST use the provided tools immediately. Do not ask for permission unless the tool explicitly requires it. Do not just list files if asked to create them. EXECUTE. WE are in " + snapshot.WorkingDir + ". Assume this is the project root unless specified otherwise.")
	}

	b.WriteString("\nUSER PROMPT:\n")
	b.WriteString(userText)
	b.WriteString("\n")

	return b.String()
}

func (s *System) maybeRecommend(ctx context.Context, intent Intent, userText string, wd string) ([]Recommendation, error) {
	if s.cfg == nil || !s.cfg.Prompt.RecommendationsEnabled {
		return nil, nil
	}
	if s.recommender == nil {
		return nil, nil
	}
	if s.cfg.Prompt.RecommendationsMaxPerRun > 0 && s.recoUsed >= s.cfg.Prompt.RecommendationsMaxPerRun {
		return nil, nil
	}

	// Only recommend for codebase-relevant intents.
	if intent != IntentPlan && intent != IntentCRUD {
		return nil, nil
	}

	// Sampling: keep this extremely low by default.
	prob := s.cfg.Prompt.RecommendationsSampleRate
	if prob <= 0 {
		prob = 0.05
	}
	// Deterministic-ish sampling: hashless, time-bucket based.
	if (time.Now().UnixNano() % 1000) > int64(prob*1000) {
		return nil, nil
	}

	s.recoUsed++
	return s.recommender.Recommend(ctx, RecommendInput{Intent: intent, UserText: userText, WorkingDir: wd, Time: time.Now()})
}
