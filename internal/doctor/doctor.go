package doctor

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"

	"github.com/nathfavour/vibeauracle/sys"
)

type HealthScore int

const (
	HealthUnknown HealthScore = iota
	HealthGood
	HealthDegraded
	HealthCritical
	HealthCatastrophic
)

// Signal represents a monitored event
type SignalType string

const (
	SignalHeartbeat SignalType = "heartbeat"
	SignalWarning   SignalType = "warning"
	SignalError     SignalType = "error"
	SignalPanic     SignalType = "panic"
	SignalInit      SignalType = "init"
	SignalCrash     SignalType = "crash"
)

// Cue allows modules to signal their status
type Cue struct {
	Source    string     `json:"source"`
	Type      SignalType `json:"type"`
	Message   string     `json:"message"`
	Timestamp time.Time  `json:"timestamp"`
	Extra     any        `json:"extra,omitempty"`
}

var (
	cues     = make(chan Cue, 100)
	mu       sync.Mutex
	logCache []Cue
)

// Start begins the monitoring loop
func Start() {
	go monitor()
}

func monitor() {
	for cue := range cues {
		mu.Lock()
		logCache = append(logCache, cue)
		if len(logCache) > 1000 {
			logCache = logCache[1:]
		}

		// If catastrophe, take action immediately?
		// For now, we just log.
		mu.Unlock()
	}
}

// Send emits a cue to the doctor
func Send(source string, typ SignalType, msg string, extra any) {
	select {
	case cues <- Cue{
		Source:    source,
		Type:      typ,
		Message:   msg,
		Timestamp: time.Now(),
		Extra:     extra,
	}:
	default:
		// Don't block if doctor is overwhelmed (potentially dying anyway)
	}
}

// Recover is a top-level deferred function to catch panics and save crash state
func Recover() {
	if r := recover(); r != nil {
		stack := string(debug.Stack())
		err := fmt.Errorf("panic: %v", r)

		fmt.Println("\n\033[31m!!! CRITICAL SYSTEM FAILURE DETECTED !!!\033[0m")
		fmt.Printf("Analyzing trauma: %v\n", err)

		// Log the crash
		logPath, _ := LogCrash(err, stack)
		fmt.Printf("\nDetailed autopsy report saved to:\n\033[1m%s\033[0m\n", logPath)

		// Run diagnosis
		health := AnalyzeHealth()
		if health == HealthCatastrophic {
			fmt.Println("\n\033[33mSystem Health: CATASTROPHIC. Multiple recent crashes detected.\033[0m")
			fmt.Println("Attempting emergency rollback protocol...")
			// Trigger rollback (shell out to self)
			// TODO: Implement direct function call if possible, or exec
		}

		// Check for technical user
		if IsTechnicalUser() {
			fmt.Println("\n\033[36m(Technical User Detected)\033[0m")
			fmt.Println("Please run: \033[1mvibeaura issue --log " + logPath + "\033[0m")
		}

		os.Exit(1)
	}
}

// LogCrash writes a crash report to AppData
func LogCrash(err error, stack string) (string, error) {
	cm, _ := sys.NewConfigManager()
	base := cm.GetDataPath("crash_logs")
	_ = os.MkdirAll(base, 0755)

	filename := fmt.Sprintf("crash_%s.json", time.Now().Format("20060102_150405"))
	path := filepath.Join(base, filename)

	report := map[string]interface{}{
		"error":     err.Error(),
		"stack":     stack,
		"timestamp": time.Now(),
		"cues":      logCache, // Include recent context
		"health":    AnalyzeHealth(),
	}

	bytes, _ := json.MarshalIndent(report, "", "  ")
	_ = os.WriteFile(path, bytes, 0644)

	// Also update config crash counters
	cfg, err := cm.Load()
	if err == nil {
		cfg.Health.CrashCount++
		cfg.Health.LastCrash = time.Now()
		cm.Save(cfg) // Best effort save
	}

	return path, nil
}

// AnalyzeHealth looks at crash history to determine system state
func AnalyzeHealth() HealthScore {
	cm, _ := sys.NewConfigManager()
	cfg, err := cm.Load()
	if err != nil {
		return HealthUnknown
	}

	// Reset counter if it's been a while (e.g. 1 hour)
	if time.Since(cfg.Health.LastCrash) > 1*time.Hour {
		if cfg.Health.CrashCount > 0 {
			// Decay
			cfg.Health.CrashCount = 0
			cm.Save(cfg)
		}
		return HealthGood
	}

	if cfg.Health.CrashCount >= 3 {
		return HealthCatastrophic
	}
	if cfg.Health.CrashCount >= 1 {
		return HealthDegraded
	}

	return HealthGood
}

// IsTechnicalUser guesses if the user is a dev
func IsTechnicalUser() bool {
	// Check for go installation
	_, err := exec.LookPath("go")
	if err == nil {
		return true
	}
	// Check for git
	_, err = exec.LookPath("git")
	if err == nil {
		return true
	}
	return false
}
