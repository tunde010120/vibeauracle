# Contributing to vibeauracle 🌌

First off, thank you for considering contributing to **vibeauracle**! We want this to be the most fluid, "God-tier" developer experience, and that only happens with your input.

We have a **Zero Friction Policy**. Whether you're fixing a typo, optimizing a cognitive loop, or adding a completely new "Vibe," your contribution is welcome.

---

## 🛠️ How to Contribute

1.  **Fork the repo** and create your branch 
2.  **Make your changes.** Don't worry about being "perfect"—we value progress and vibes over strict bureaucracy.
3.  **Open a Pull Request.** We'll review it and get it merged.

---

## 🎭 Creating Your Own "Vibe" (Plugins)

A **Vibe** is a community-contributed module that adds specialized skills to the vibeauracle brain. Adding one is easy:

### 1. Create your Vibe directory
Navigate to the `vibes/` directory and create a new folder for your project:
```bash
mkdir -p vibes/my-cool-vibe
cd vibes/my-cool-vibe
go mod init github.com/nathfavour/vibeauracle/vibes/my-cool-vibe
```

### 2. Implement the Vibe Interface
Create a `main.go` and implement the `Vibe` interface from `pkg/vibe`:

```go
package main

import (
	"context"
	"fmt"
	"github.com/nathfavour/vibeauracle/pkg/vibe"
)

type MyCoolVibe struct{}

func (v *MyCoolVibe) Name() string { return "my-cool-vibe" }
func (v *MyCoolVibe) Description() string { return "Does something epic" }

func (v *MyCoolVibe) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// Your agentic logic here
	return "Vibe executed!", nil
}

// Ensure it matches the interface
var _ vibe.Vibe = (*MyCoolVibe)(nil)
```

### 3. Register your Vibe
Add your module to the root `go.work` file:
```bash
go work use ./vibes/my-cool-vibe
```

---

## 🧠 Agent Skills
If you want to add a specific "Skill" (a functional action the brain can take), you can contribute directly to `internal/brain` or define it within your Vibe using the `Skill` struct in `pkg/vibe`.

## 📜 Code of Conduct
Be excellent to each other. That's the only rule.

## 🚀 Speed over Bureaucracy
If you have a cool idea, just build it. We'd rather see a messy working prototype than a perfect proposal that never gets written. 

Let's build the Alpha & Omega. 🏗️

