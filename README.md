<div align="center">
  <img src="./assets/vibeaura.png" width="128" alt="Vibeauracle Logo" />

  # vibeauracle
  **Distributed, System-Intimate AI Engineering Ecosystem**

  [![Stable](https://img.shields.io/badge/Stable-ec1de87-10B981?style=for-the-badge&logo=git&logoColor=white)](https://github.com/nathfavour/vibeauracle/tree/release)
  [![Beta](https://img.shields.io/badge/Beta-7ebb650-7C3AED?style=for-the-badge&logo=git&logoColor=white)](https://github.com/nathfavour/vibeauracle/tree/master)
  [![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev)
  [![License](https://img.shields.io/badge/License-MIT-F59E0B?style=for-the-badge)](LICENSE)
</div>

---

## 🌌 The Vision
**vibeauracle** unifies the terminal, the IDE, and the AI assistant into a single, keyboard-centric interface. It's built for engineers who value **system-intimacy**—where the AI isn't just a chatbot, but a co-pilot with deep access to your hardware, configuration, and environment.

## ⚡ Core Features
- **Keyboard-Centric**: Designed for speed and fluid terminal workflows.
- **Deep Integration**: Real-time awareness of system stats, files, and Git state.
- **Universal Updates**: Seamless, background updates tracking Git SHAs directly.
- **Multi-Provider**: Works with Ollama, OpenAI, and GitHub Models out of the box.
- **Cleanup First**: First-class `uninstall` support to preserve privacy and system hygiene.

---

## 🚀 Quick Install

### 🐧 Linux / 🍎 macOS / 🤖 Android
```bash
curl -fsSL https://raw.githubusercontent.com/nathfavour/vibeauracle/release/install.sh | sh
```

### 🪟 Windows
```powershell
iex (irm https://raw.githubusercontent.com/nathfavour/vibeauracle/release/install.ps1)
```

> **Pro Tip:** One install is all you need. Keep the vibes fresh with `vibeaura update`.

---

## 🛠️ Usage
| Command | Description |
|---------|-------------|
| `vibeaura` | Launch the interactive session |
| `vibeaura update` | Pull the latest SHA from your current branch |
| `vibeaura sys stats` | View real-time system power snapshot |
| `vibeaura models` | Discover and switch AI providers |

### 🗑️ Uninstall & Clean
We respect your space. To remove the tool but keep your data:
```bash
vibeaura uninstall
```
To wipe **everything** (binary + secrets + config):
```bash
vibeaura uninstall --clean
```

---

## 🤝 Contributing
We love community "Vibes"! Check out [CONTRIBUTING.md](./CONTRIBUTING.md) to add your own agent skills.

<div align="center">
  <sub>Built with 💜 for the future of engineering.</sub>
</div>
