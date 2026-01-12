# 🌀 VIBES: The Extension API

**Vibes** are the soul of vibeauracle's extensibility. They are natural-language-powered, markdown-defined extensions that can modify, extend, or completely redefine any aspect of the system—from UI colors to update schedules to agent behavior.

> **Philosophy:** Treat vibeauracle like a microkernel. The core provides primitives; Vibes provide policy.

---

## 📜 Vibe Anatomy

A Vibe is a `.vibe.md` file placed in `~/.vibeauracle/vibes/` or any registered vibe directory. It uses a strict YAML front matter + Markdown body format.

```markdown
---
name: my-custom-vibe
version: 1.0.0
author: nathfavour
hooks:
  - on_startup
  - on_file_change
  - on_schedule
permissions:
  - config.write
  - ui.theme
  - scheduler.create
schedule: "*/5 * * * *"  # Cron syntax (optional)
---

# My Custom Vibe

This vibe does XYZ...

## Instructions

When activated, perform the following:
1. Change the TUI accent color to `#FF5733`
2. Every 5 minutes, run a git status check
3. If there are uncommitted changes, notify me
```

---

## ⚡ Core Concepts

### 1. Hooks
Vibes attach to lifecycle events:

| Hook | Trigger |
|------|---------|
| `on_startup` | When vibeaura launches |
| `on_shutdown` | When vibeaura exits |
| `on_file_change` | When the watcher detects a file event |
| `on_command` | Before/after any CLI command |
| `on_tool_call` | Before/after any tool execution |
| `on_schedule` | At the specified cron time |
| `on_config_change` | When any config value changes |
| `on_model_response` | When the AI returns a response |
| `on_update` | Before/after an update is applied |

### 2. Permissions
Vibes must declare what they need:

| Permission | Access |
|------------|--------|
| `config.read` | Read user configuration |
| `config.write` | Modify user configuration |
| `ui.theme` | Change TUI colors/layout |
| `ui.layout` | Modify TUI arrangement |
| `scheduler.create` | Create scheduled tasks |
| `scheduler.cancel` | Cancel scheduled tasks |
| `agent.prompt` | Modify system prompts |
| `agent.tools` | Register/unregister tools |
| `agent.lock` | Lock agent behind password |
| `update.frequency` | Change auto-update schedule |
| `update.channel` | Switch between stable/beta |
| `binary.self_modify` | Rebuild/patch the binary |
| `system.shell` | Execute shell commands |
| `system.fs` | Read/write filesystem |

### 3. Natural Language Instructions
The Markdown body is parsed by the AI to understand intent. This enables:
- Zero-code extension development
- Human-readable configuration
- AI-assisted debugging and optimization

---

## 🗓️ Scheduler

Vibes can schedule recurring or one-shot tasks:

```yaml
schedule: "0 9 * * *"  # Every day at 9 AM
schedule_once: "2026-01-15T10:00:00Z"  # One-time trigger
```

The scheduler supports:
- **Cron expressions** for recurring tasks
- **ISO 8601 timestamps** for one-shot events
- **Relative times** like `in 5m`, `in 2h`

---

## 🔒 Security Model

### Agent Locking
```yaml
security:
  require_password: true
  password_hash: "sha256:..."
  lock_after: 5m  # Lock after 5 minutes of inactivity
```

### Permission Escalation
Vibes requiring sensitive permissions will prompt for approval on first run.

### Sandboxing
Vibes run in an isolated context by default. They can request:
- `sandbox.escape` - Full system access (requires explicit approval)

---

## 🎨 UI Customization

```yaml
ui:
  theme:
    primary: "#7C3AED"
    secondary: "#06B6D4"
    accent: "#F59E0B"
    background: "#0D0D0D"
  layout:
    sidebar: left
    tree_width: 30%
```

---

## 🛠️ Defining Custom Tools

Vibes can register new tools for the AI:

```yaml
tools:
  - name: check_weather
    description: "Get current weather for a city"
    parameters:
      city:
        type: string
        required: true
    action: |
      curl -s "wttr.in/${city}?format=3"
```

---

## 📦 Vibe Lifecycle

1. **Discovery**: Vibes are scanned from registered directories
2. **Validation**: YAML front matter and permissions are verified
3. **Registration**: Hooks are attached to internal events
4. **Activation**: Vibe instructions are parsed and applied
5. **Execution**: Scheduled/triggered actions run in context

---

## 🌍 Sharing Vibes

Vibes can be shared via:
- Git repositories
- Direct file transfer
- Future: A central Vibe registry

```bash
vibeaura vibe install https://github.com/user/my-vibe
vibeaura vibe enable my-vibe
vibeaura vibe disable my-vibe
vibeaura vibe list
```

---

## 🧬 Advanced: Binary Self-Modification

For ultimate control, Vibes with `binary.self_modify` permission can:
- Rebuild the binary with custom ldflags
- Inject compile-time constants
- Add new CLI commands

```yaml
binary:
  ldflags:
    - "-X main.CustomVar=value"
  rebuild_on: config_change
```

---

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────┐
│                     VIBEAURACLE CORE                    │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐   │
│  │  Brain   │ │ Watcher  │ │ Scheduler│ │  Config  │   │
│  └────┬─────┘ └────┬─────┘ └────┬─────┘ └────┬─────┘   │
│       │            │            │            │          │
│  ─────┴────────────┴────────────┴────────────┴──────    │
│                      EVENT BUS                          │
│  ───────────────────────────────────────────────────    │
│       │            │            │            │          │
│  ┌────┴────┐  ┌────┴────┐  ┌────┴────┐  ┌────┴────┐    │
│  │ Vibe A  │  │ Vibe B  │  │ Vibe C  │  │ Vibe D  │    │
│  └─────────┘  └─────────┘  └─────────┘  └─────────┘    │
└─────────────────────────────────────────────────────────┘
```

Every component exposes hooks. Every hook is a "female connector" that Vibes can plug into.

---

## 🚀 Getting Started

1. Create `~/.vibeauracle/vibes/hello.vibe.md`
2. Add front matter with `hooks: [on_startup]`
3. Write natural language instructions
4. Run `vibeaura` - your Vibe is live!

---

<div align="center">
  <sub>Vibes are the future of personal AI customization.</sub>
</div>
