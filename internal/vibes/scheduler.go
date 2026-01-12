package vibes

import (
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// ScheduledTask represents a task that runs on a schedule.
type ScheduledTask struct {
	ID       cron.EntryID
	VibeName string
	Schedule string
	Action   func()
}

// Scheduler manages cron-based and one-shot scheduled tasks.
type Scheduler struct {
	mu       sync.RWMutex
	cron     *cron.Cron
	tasks    map[string][]ScheduledTask
	oneshots map[string]*time.Timer
}

// NewScheduler creates a new task scheduler.
func NewScheduler() *Scheduler {
	return &Scheduler{
		cron:     cron.New(cron.WithSeconds()),
		tasks:    make(map[string][]ScheduledTask),
		oneshots: make(map[string]*time.Timer),
	}
}

// Start begins the scheduler.
func (s *Scheduler) Start() {
	s.cron.Start()
}

// Stop halts the scheduler.
func (s *Scheduler) Stop() {
	s.cron.Stop()

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, timer := range s.oneshots {
		timer.Stop()
	}
	s.oneshots = make(map[string]*time.Timer)
}

// Schedule adds a recurring task based on a cron expression.
func (s *Scheduler) Schedule(vibeName, cronExpr string, action func()) (cron.EntryID, error) {
	entryID, err := s.cron.AddFunc(cronExpr, action)
	if err != nil {
		return 0, err
	}

	task := ScheduledTask{
		ID:       entryID,
		VibeName: vibeName,
		Schedule: cronExpr,
		Action:   action,
	}

	s.mu.Lock()
	s.tasks[vibeName] = append(s.tasks[vibeName], task)
	s.mu.Unlock()

	return entryID, nil
}

// ScheduleOnce adds a one-shot task at a specific time.
func (s *Scheduler) ScheduleOnce(vibeName string, at time.Time, action func()) error {
	duration := time.Until(at)
	if duration < 0 {
		return nil // Already passed
	}

	timer := time.AfterFunc(duration, func() {
		action()

		s.mu.Lock()
		delete(s.oneshots, vibeName+at.String())
		s.mu.Unlock()
	})

	s.mu.Lock()
	s.oneshots[vibeName+at.String()] = timer
	s.mu.Unlock()

	return nil
}

// ScheduleIn adds a task that runs after a relative duration.
func (s *Scheduler) ScheduleIn(vibeName string, d time.Duration, action func()) {
	timer := time.AfterFunc(d, func() {
		action()

		s.mu.Lock()
		delete(s.oneshots, vibeName+d.String())
		s.mu.Unlock()
	})

	s.mu.Lock()
	s.oneshots[vibeName+d.String()] = timer
	s.mu.Unlock()
}

// Cancel removes all scheduled tasks for a Vibe.
func (s *Scheduler) Cancel(vibeName string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Cancel cron tasks
	if tasks, ok := s.tasks[vibeName]; ok {
		for _, task := range tasks {
			s.cron.Remove(task.ID)
		}
		delete(s.tasks, vibeName)
	}

	// Cancel one-shot timers (by prefix match)
	for key, timer := range s.oneshots {
		if len(key) >= len(vibeName) && key[:len(vibeName)] == vibeName {
			timer.Stop()
			delete(s.oneshots, key)
		}
	}
}

// ListTasks returns all active scheduled tasks for a Vibe.
func (s *Scheduler) ListTasks(vibeName string) []ScheduledTask {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if tasks, ok := s.tasks[vibeName]; ok {
		result := make([]ScheduledTask, len(tasks))
		copy(result, tasks)
		return result
	}
	return nil
}

// NextRun returns the next execution time for a Vibe's tasks.
func (s *Scheduler) NextRun(vibeName string) *time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if tasks, ok := s.tasks[vibeName]; ok && len(tasks) > 0 {
		entry := s.cron.Entry(tasks[0].ID)
		if !entry.Next.IsZero() {
			return &entry.Next
		}
	}
	return nil
}
