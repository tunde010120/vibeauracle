module github.com/nathfavour/vibeauracle/cmd/vibeaura

go 1.21

require (
	github.com/charmbracelet/bubbles v0.21.0
	github.com/charmbracelet/bubbletea v1.3.10
	github.com/charmbracelet/lipgloss v1.1.0
	github.com/google/uuid v1.6.0
	github.com/mattn/go-runewidth v0.0.16
	github.com/nathfavour/vibeauracle/brain v0.0.0-00010101000000-000000000000
	github.com/nathfavour/vibeauracle/internal/doctor v0.0.0-00010101000000-000000000000
	github.com/nathfavour/vibeauracle/sys v0.0.0
	github.com/nathfavour/vibeauracle/tooling v0.0.0-00010101000000-000000000000
	github.com/spf13/cobra v1.10.2
	golang.org/x/mod v0.32.0
)

require (
	github.com/99designs/go-keychain v0.0.0-20191008050251-8e49817e8af4 // indirect
	github.com/99designs/keyring v1.2.2 // indirect
	github.com/atotto/clipboard v0.1.4 // indirect
	github.com/aymanbagabas/go-osc52/v2 v2.0.1 // indirect
	github.com/charmbracelet/colorprofile v0.2.3-0.20250311203215-f60798e515dc // indirect
	github.com/charmbracelet/x/ansi v0.10.1 // indirect
	github.com/charmbracelet/x/cellbuf v0.0.13-0.20250311204145-2c3ea96c31dd // indirect
	github.com/charmbracelet/x/term v0.2.1 // indirect
	github.com/danieljoos/wincred v1.1.2 // indirect
	github.com/dlclark/regexp2 v1.11.4 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/dvsekhvalnov/jose2go v1.7.0 // indirect
	github.com/erikgeiser/coninput v0.0.0-20211004153227-1c3628e74d0f // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/glebarez/go-sqlite v1.22.0 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/godbus/dbus v0.0.0-20190726142602-4481cbc300e2 // indirect
	github.com/gsterjov/go-libsecret v0.0.0-20161001094733-a6f4afe4910c // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20250317134145-8bc96cf8fc35 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-localereader v0.0.1 // indirect
	github.com/mtibben/percent v0.2.1 // indirect
	github.com/muesli/ansi v0.0.0-20230316100256-276c6243b2f6 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/muesli/termenv v0.16.0 // indirect
	github.com/nathfavour/vibeauracle/auth v0.0.0-00010101000000-000000000000 // indirect
	github.com/nathfavour/vibeauracle/context v0.0.0-00010101000000-000000000000 // indirect
	github.com/nathfavour/vibeauracle/model v0.0.0-00010101000000-000000000000 // indirect
	github.com/nathfavour/vibeauracle/prompt v0.0.0 // indirect
	github.com/nathfavour/vibeauracle/vault v0.0.0-00010101000000-000000000000 // indirect
	github.com/ollama/ollama v0.13.5 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/pkoukk/tiktoken-go v0.1.6 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/sagikazarmark/locafero v0.11.0 // indirect
	github.com/shirou/gopsutil/v3 v3.24.5 // indirect
	github.com/shoenig/go-m1cpu v0.1.6 // indirect
	github.com/sourcegraph/conc v0.3.1-0.20240121214520-5f936abd7ae8 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/spf13/viper v1.21.0 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/tklauser/go-sysconf v0.3.15 // indirect
	github.com/tklauser/numcpus v0.10.0 // indirect
	github.com/tmc/langchaingo v0.1.14 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.46.0 // indirect
	golang.org/x/exp v0.0.0-20250606033433-dcc06ee1d476 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/term v0.38.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	modernc.org/libc v1.37.6 // indirect
	modernc.org/mathutil v1.6.0 // indirect
	modernc.org/memory v1.7.2 // indirect
	modernc.org/sqlite v1.28.0 // indirect
)

replace github.com/nathfavour/vibeauracle/sys => ../../internal/sys

replace github.com/nathfavour/vibeauracle/brain => ../../internal/brain

replace github.com/nathfavour/vibeauracle/tooling => ../../internal/tooling

replace github.com/nathfavour/vibeauracle/internal/doctor => ../../internal/doctor

replace github.com/nathfavour/vibeauracle/auth => ../../internal/auth

replace github.com/nathfavour/vibeauracle/vault => ../../internal/vault

replace github.com/nathfavour/vibeauracle/model => ../../internal/model

replace github.com/nathfavour/vibeauracle/context => ../../internal/context

replace github.com/nathfavour/vibeauracle/prompt => ../../internal/prompt
