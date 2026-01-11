# Copilot Instructions for vibeauracle

## 🌌 The Big Picture
**vibeauracle** is a Distributed, System-Intimate AI Engineering Ecosystem. It uses a **Modular Monolith** architecture coordinated via **Go Workspaces**.

- **Core Module (`internal/brain`)**: The cognitive orchestrator managing the Plan-Execute-Reflect loop.
- **Entry Point (`cmd/vibeaura`)**: Unified CLI/TUI built with `bubbletea` and `lipgloss`.
- **System Layer (`internal/sys`)**: Hardware-aware monitoring and virtual FS operations.
- **Providers (`internal/model`)**: Abstraction for local (Ollama) and cloud (OpenAI) LLMs.

### 🏗️ Architecture Conventions
- **Go Workspaces**: The project uses `go.work`. Each directory in `internal/` and `pkg/` is its own Go module.
- **Hexagonal Design**: Keep logic in `internal/` decoupled. Use interfaces from `pkg/vibe` or within modules to interact between layers.
- **Error Handling**: Always wrap errors with context.
  ```go
  return fmt.Errorf("starting brain session: %w", err)
  ```

## 🛠️ Developer Workflow
- **Syncing Workspace**: After adding dependencies to any module, run:
  ```bash
  go work sync
  ```
- **Testing**: Run tests for specific modules to ensure hardware/system intimacy remains intact:
  ```bash
  go test ./internal/brain/... ./internal/sys/...
  ```
- **Rolling Releases**: 
    - `master`: Rolling development branch (untested/bleeding edge).
    - `release`: Stable branch (triggers production builds).
    - Version Tags: `v*` triggers a GitHub Release.

## 🗝️ Critical Patterns
### System Intimacy
When modifying `internal/sys`, ensure compatibility with **Termux (Android)** and **Arch Linux**. Check for environment markers like `/data/data/com.termux/files/usr/bin/bash` or the `TERMUX_VERSION` env var.

### Vibes & Plugins
Community modules live in `vibes/`. 
- New vibes should implement the `Vibe` interface from `pkg/vibe`.
- Register new vibes in the root `go.work` using `go work use ./vibes/your-vibe`.

### Update Pipeline
The `update` command supports binary updates and source builds.
- **Binary**: Fetches from GitHub Releases.
- **Source**: Clones to `~/.vibeauracle/source`, builds with `GOTOOLCHAIN=local`, and replaces the current executable.
- Use `vibeaura update --list-assets` to verify release assets before manual troubleshooting.

### The Brain Loop
The `vibe-brain` implements a recursive agentic loop:
1. **Perceive**: Snapshot system resources (VRAM/CPU).
2. **Plan**: Generate a Chain of Thought.
3. **Execute**: Delegate to MCP or Sys tools.
4. **Reflect**: Analyze stderr/results and self-correct.

## 🎨 TUI Design (Bubble Tea)
- Use `lipgloss` for all styling.
- Follow the **Elm Architecture**: `Init()`, `Update()`, `View()`.
- Ensure the TUI is responsive to terminal resizing.
