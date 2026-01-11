package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nathfavour/vibeauracle/prompt"
	"github.com/nathfavour/vibeauracle/tooling"
)

// Goal represents the high-level objective of an agentic loop.
type Goal struct {
	ID          string
	Description string
	Status      string // pending|active|completed|failed
	Milestones  []Milestone
	Confidence  float64
}

type Milestone struct {
	Description string
	Completed   bool
}

// LoopState tracks the progress of a multi-turn interaction.
type LoopState struct {
	Goal       Goal
	Turns      int
	MaxTurns   int
	History    []string
	Confidence float64
	StartTime  time.Time
}

// Model defines the minimal interface the agent needs to prompt the AI.
type Model interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

// Engine manages the "handshake" loop between AI creativity and agentic control.
type Engine struct {
	model    Model
	registry *tooling.Registry
	prompts  *prompt.System
	config   Config
}

type Config struct {
	MaxTurns       int
	MinConfidence  float64
	LearningEnabled bool
}

func NewEngine(m Model, r *tooling.Registry, p *prompt.System, cfg Config) *Engine {
	if cfg.MaxTurns == 0 {
		cfg.MaxTurns = 10
	}
	if cfg.MinConfidence == 0 {
		cfg.MinConfidence = 0.3
	}
	return &Engine{
		model:    m,
		registry: r,
		prompts:  p,
		config:   cfg,
	}
}

// Run executes a task loop until completion, turn limit, or confidence drop.
func (e *Engine) Run(ctx context.Context, initialPrompt string, onUpdate func(LoopState)) (string, error) {
	state := LoopState{
		Goal: Goal{
			Description: initialPrompt,
			Status:      "active",
		},
		MaxTurns:   e.config.MaxTurns,
		Confidence: 1.0,
		StartTime:  time.Now(),
	}

	for state.Turns < state.MaxTurns {
		state.Turns++
		if onUpdate != nil {
			onUpdate(state)
		}

		// 1. Handshake: Build current prompt based on state.
		// In agent mode, the prompt metamorphoses into a "work instruction".
		handshakePrompt := e.buildHandshakePrompt(state)

		// 2. AI (Bricklayer) Generation.
		resp, err := e.model.Generate(ctx, handshakePrompt)
		if err != nil {
			return "", fmt.Errorf("agent turn %d: %w", state.Turns, err)
		}

		// 3. Analysis: The bureaucratic manager parses the bricks.
		parsed := prompt.ParseModelResponse(resp)
		
		// 4. Execution Loop: Extract and run tool calls if any.
		result, toolsCalled, err := e.executeInferredTools(ctx, parsed)
		if err != nil {
			// If we hit an approval error, we must bubble it up as an "intervention required" signal.
			return "", err
		}

		// 5. Reflection & Confidence Adjustment.
		state.History = append(state.History, resp)
		if result != "" {
			state.History = append(state.History, "TOOL_RESULT: "+result)
		}
		
		state.Confidence = e.calculateConfidence(state, toolsCalled)

		// Check for exit conditions.
		if state.Confidence < e.config.MinConfidence {
			return resp, fmt.Errorf("agent lost confidence (%.2f < %.2f) - consulting user", state.Confidence, e.config.MinConfidence)
		}

		// Look for completion markers in AI response.
		if strings.Contains(strings.ToLower(resp), "goal completed") || strings.Contains(strings.ToLower(resp), "[task_done]") {
			state.Goal.Status = "completed"
			return resp, nil
		}

		// If no tools were called and no goal was met, the bricklayer might be stuck.
		if !toolsCalled && state.Turns > 2 {
			state.Confidence -= 0.2
		}
	}

	return "", fmt.Errorf("max turns (%d) reached without completing goal", state.MaxTurns)
}

func (e *Engine) buildHandshakePrompt(state LoopState) string {
	// Multi-layered handshake: Goal + History + Rules + Current State.
	return fmt.Sprintf(`### AGENT WORK LOOP (Turn %d/%d)
GOAL: %s
CONFIDENCE: %.2f

### RULES:
- If a task is finished, include "[TASK_DONE]" or "GOAL COMPLETED".
- Use tools available to perform CRUD or system tasks.
- If you need clarification from the client, ask directly.

### WORK HISTORY:
%s

### CURRENT ACTION:
Analyze history and continue working towards the goal.`, 
		state.Turns, state.MaxTurns, state.Goal.Description, state.Confidence, strings.Join(state.History, "\n---\n"))
}

func (e *Engine) calculateConfidence(state LoopState, toolsCalled bool) float64 {
	score := state.Confidence

	// Penalities
	if state.Turns > (state.MaxTurns / 2) {
		score -= 0.05 // Fatigue
	}

	// Detected loop in history (halucination/stuck)
	if len(state.History) >= 2 {
		last := state.History[len(state.History)-1]
		prev := state.History[len(state.History)-2]
		if last == prev {
			score -= 0.4
		}
	}

	// Reward progress (not implemented deeply here, but placeholders)
	if toolsCalled {
		score += 0.05
	}

	if score > 1.0 { score = 1.0 }
	if score < 0.0 { score = 0.0 }
	return score
}

func (e *Engine) executeInferredTools(ctx context.Context, parsed prompt.ParsedResponse) (string, bool, error) {
	// Simple heuristic: search for tool-like patterns in text or specifically formatted blocks.
	// For now, we rely on standard tooling registry lookups if we find structured commands.
	
	// Implementation note: This is where we'd parse things like `USE sys_write_file {"path": "..."}`
	// or rely on the bricks (AI) being trained to output valid tool calls.
	
	return "", false, nil
}

import "strings"
