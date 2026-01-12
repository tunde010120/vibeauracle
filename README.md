# 🏗️ vibeauracle: The Alpha & Omega

**vibeauracle** is a **Distributed, System-Intimate AI Engineering Ecosystem**. It unifies the terminal, the IDE, and the AI assistant into a single, keyboard-centric interface.

## 🚀 Installation

You can install **vibeauracle** quickly using the command for your platform. The installer will automatically detect your architecture and fetch the latest stable release.

### 🐧 Linux / 🍎 macOS / 🤖 Android (Bash)
```bash
curl -fsSL https://raw.githubusercontent.com/nathfavour/vibeauracle/release/install.sh | sh
```
*Or using wget:*
```bash
wget -qO- https://raw.githubusercontent.com/nathfavour/vibeauracle/release/install.sh | sh
```

### 🪟 Windows (Powershell)
Run the following in PowerShell as Administrator:
```powershell
iex (irm https://raw.githubusercontent.com/nathfavour/vibeauracle/release/install.ps1)
```

### 📦 Manual Installation (Pre-built)
If you prefer, you can download pre-built binaries directly from our [Releases page](https://github.com/nathfavour/vibeauracle/releases).

Simply choose the binary that matches your OS and architecture, make it executable (`chmod +x`), and move it to your PATH. **You only need to do this once**—future updates are handled automatically with:
```bash
vibeaura update
```

### 🐳 Docker
You can also run **vibeauracle** in a containerized environment:
```bash
docker-compose up -d
docker-compose run vibeaura
```

## 🗑️ Uninstall

If you need to remove **vibeauracle** from your system, you can do so cleanly:

**Remove the binary (keeps configuration/data):**
```bash
vibeaura uninstall
```

**Full wipe (binary + all application data/secrets):**
```bash
vibeaura uninstall --clean
```

## 🧪 Testing
Run the unit tests to ensure everything is working correctly:
```bash
go test ./internal/brain/... ./internal/model/... ./internal/sys/...
```

## 🧩 Supported Platforms
* **Linux**: amd64, arm64 (Arch, Debian, etc.)
* **macOS**: Apple Silicon (M1/M2/M3), Intel
* **Windows**: amd64, arm64 (via PowerShell/WSL)
* **Android**: arm64 (via Termux)

## 🛠️ Getting Started
Once installed, start a session:
```bash
vibeaura
```

## 🏗️ Architecture
For deep technical details, see [ARCHITECTURE.md](./ARCHITECTURE.md).

## 🤝 Contributing
We love community "Vibes"! Check out [CONTRIBUTING.md](./CONTRIBUTING.md) to see how you can add your own agent skills and plugins.
