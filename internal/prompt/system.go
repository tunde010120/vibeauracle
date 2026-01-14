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

	// Base system layer - ACTION FIRST
	layers = append(layers, "You are vibe auracle's core assistant. You are an EXECUTOR, not a conversationalist.")
	layers = append(layers, "NEVER ask clarifying questions. NEVER ask for permission. NEVER explain what you're about to do.")
	layers = append(layers, "If the user's request has typos or is unclear, interpret the most likely intent and ACT on it immediately.")
	layers = append(layers, "Explanations are ONLY given when explicitly requested with words like 'explain', 'why', or 'how does'.")
	layers = append(layers, "Your default behavior is: READ the request → EXECUTE the action → REPORT the result briefly.")

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
		layers = append(layers, "MODE=ASK. Answer clearly and concisely. Keep it brief.")
	case IntentPlan:
		layers = append(layers, "MODE=PLAN. Provide a structured plan. No fluff.")
	case IntentCRUD:
		layers = append(layers, "MODE=CRUD. Execute file/code changes immediately. No narration.")
	default:
		layers = append(layers, "MODE=DO. Execute the task. Minimal output.")
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
		b.WriteString("\nAVAILABLE TOOLS:\n")
		b.WriteString(toolDefs)
		b.WriteString(`
HOW TO USE TOOLS:
You are an AGENTIC assistant. You MUST use tools to complete tasks. Do NOT just describe what you would do - ACTUALLY DO IT.

To invoke a tool, output a JSON code block with the following format:
` + "```json" + `
{"tool": "TOOL_NAME", "parameters": {"param1": "value1", "param2": "value2"}}
` + "```" + `

EXAMPLE - To create a file:
` + "```json" + `
{"tool": "sys_write_file", "parameters": {"path": "deployment.yaml", "content": "apiVersion: apps/v1\nkind: Deployment..."}}
` + "```" + `

EXAMPLE - To read a file:
` + "```json" + `
{"tool": "sys_read_file", "parameters": {"path": "README.md"}}
` + "```" + `

CRITICAL RULES:
1. DO NOT ask for permission to use tools - just use them.
2. DO NOT say "I will now create the file" - instead, OUTPUT THE JSON TOOL CALL.
3. DO NOT ask "Did you mean...?" - interpret typos and act on the most likely intent.
4. DO NOT explain what you're about to do - just do it.
5. If the user asks you to create/modify/read files, you MUST output a tool call IMMEDIATELY.
6. After the tool executes, report the result in ONE sentence maximum.
7. Current working directory is: ` + snapshot.WorkingDir + `

`)
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
