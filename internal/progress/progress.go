package progress

import (
	"fmt"
	"sync"
	"time"

	"crawlr/internal/logger"
)

// ProgressReporter represents a progress reporting system
type ProgressReporter struct {
	logger        *logger.Logger
	operation     string
	total         int
	current       int
	startTime     time.Time
	lastUpdate    time.Time
	updateMutex   sync.Mutex
	complete      bool
	completeChan  chan bool
	progressSteps []ProgressStep
}

// ProgressStep represents a step in the progress
type ProgressStep struct {
	Name        string
	Description string
	StartTime   time.Time
	EndTime     time.Time
	Completed   bool
	Error       error
}

// NewProgressReporter creates a new progress reporter
func NewProgressReporter(logger *logger.Logger, operation string, total int) *ProgressReporter {
	return &ProgressReporter{
		logger:       logger,
		operation:    operation,
		total:        total,
		startTime:    time.Now(),
		lastUpdate:   time.Now(),
		completeChan: make(chan bool, 1),
	}
}

// Increment increments the progress counter
func (p *ProgressReporter) Increment() {
	p.updateMutex.Lock()
	defer p.updateMutex.Unlock()

	p.current++
	p.lastUpdate = time.Now()

	// Log progress every 5% or every 10 items, whichever is more frequent
	if p.total > 0 {
		percentage := (p.current * 100) / p.total
		if percentage%5 == 0 || p.current%10 == 0 || p.current == p.total {
			p.logger.Progress(p.operation, p.current, p.total)
		}
	} else {
		// If total is unknown, log every 10 items
		if p.current%10 == 0 {
			p.logger.Progress(p.operation, p.current, p.total)
		}
	}
}

// SetTotal sets the total number of items
func (p *ProgressReporter) SetTotal(total int) {
	p.updateMutex.Lock()
	defer p.updateMutex.Unlock()

	p.total = total
	p.logger.Progress(p.operation, p.current, p.total)
}

// SetCurrent sets the current progress
func (p *ProgressReporter) SetCurrent(current int) {
	p.updateMutex.Lock()
	defer p.updateMutex.Unlock()

	p.current = current
	p.lastUpdate = time.Now()
	p.logger.Progress(p.operation, p.current, p.total)
}

// GetProgress returns the current progress
func (p *ProgressReporter) GetProgress() (int, int) {
	p.updateMutex.Lock()
	defer p.updateMutex.Unlock()

	return p.current, p.total
}

// GetPercentage returns the progress as a percentage
func (p *ProgressReporter) GetPercentage() int {
	p.updateMutex.Lock()
	defer p.updateMutex.Unlock()

	if p.total <= 0 {
		return 0
	}
	return (p.current * 100) / p.total
}

// GetElapsedTime returns the elapsed time since the progress reporter was created
func (p *ProgressReporter) GetElapsedTime() time.Duration {
	p.updateMutex.Lock()
	defer p.updateMutex.Unlock()

	return time.Since(p.startTime)
}

// GetEstimatedTimeRemaining returns the estimated time remaining
func (p *ProgressReporter) GetEstimatedTimeRemaining() time.Duration {
	p.updateMutex.Lock()
	defer p.updateMutex.Unlock()

	if p.total <= 0 || p.current <= 0 {
		return 0
	}

	elapsed := time.Since(p.startTime)
	remaining := (elapsed / time.Duration(p.current)) * time.Duration(p.total-p.current)
	return remaining
}

// Complete marks the progress as complete
func (p *ProgressReporter) Complete() {
	p.updateMutex.Lock()
	defer p.updateMutex.Unlock()

	if !p.complete {
		p.complete = true
		p.current = p.total
		p.lastUpdate = time.Now()
		elapsed := time.Since(p.startTime)

		p.logger.Info(fmt.Sprintf("Progress completed: %s - %d/%d in %v",
			p.operation, p.current, p.total, elapsed.Round(time.Millisecond)))

		// Notify any listeners that progress is complete
		select {
		case p.completeChan <- true:
		default:
		}
	}
}

// IsComplete returns whether the progress is complete
func (p *ProgressReporter) IsComplete() bool {
	p.updateMutex.Lock()
	defer p.updateMutex.Unlock()

	return p.complete
}

// WaitForCompletion waits for the progress to be complete
func (p *ProgressReporter) WaitForCompletion() {
	<-p.completeChan
}

// AddStep adds a progress step
func (p *ProgressReporter) AddStep(name, description string) {
	p.updateMutex.Lock()
	defer p.updateMutex.Unlock()

	step := ProgressStep{
		Name:        name,
		Description: description,
		StartTime:   time.Now(),
	}
	p.progressSteps = append(p.progressSteps, step)

	p.logger.Info(fmt.Sprintf("Progress step started: %s - %s", name, description))
}

// CompleteStep marks a progress step as complete
func (p *ProgressReporter) CompleteStep(name string, err error) {
	p.updateMutex.Lock()
	defer p.updateMutex.Unlock()

	for i, step := range p.progressSteps {
		if step.Name == name && !step.Completed {
			p.progressSteps[i].EndTime = time.Now()
			p.progressSteps[i].Completed = true
			p.progressSteps[i].Error = err

			duration := p.progressSteps[i].EndTime.Sub(step.StartTime)
			if err != nil {
				p.logger.Error(fmt.Sprintf("Progress step failed: %s - %s (error: %v, duration: %v)",
					name, step.Description, err, duration.Round(time.Millisecond)))
			} else {
				p.logger.Info(fmt.Sprintf("Progress step completed: %s - %s (duration: %v)",
					name, step.Description, duration.Round(time.Millisecond)))
			}
			break
		}
	}
}

// GetSteps returns the progress steps
func (p *ProgressReporter) GetSteps() []ProgressStep {
	p.updateMutex.Lock()
	defer p.updateMutex.Unlock()

	// Return a copy to avoid race conditions
	steps := make([]ProgressStep, len(p.progressSteps))
	copy(steps, p.progressSteps)
	return steps
}

// GetCompletedSteps returns the number of completed steps
func (p *ProgressReporter) GetCompletedSteps() int {
	p.updateMutex.Lock()
	defer p.updateMutex.Unlock()

	count := 0
	for _, step := range p.progressSteps {
		if step.Completed {
			count++
		}
	}
	return count
}

// GetStepStatus returns the status of a specific step
func (p *ProgressReporter) GetStepStatus(name string) (bool, error) {
	p.updateMutex.Lock()
	defer p.updateMutex.Unlock()

	for _, step := range p.progressSteps {
		if step.Name == name {
			return step.Completed, step.Error
		}
	}
	return false, fmt.Errorf("step not found: %s", name)
}

// ProgressManager manages multiple progress reporters
type ProgressManager struct {
	reporters map[string]*ProgressReporter
	mutex     sync.Mutex
	logger    *logger.Logger
}

// NewProgressManager creates a new progress manager
func NewProgressManager(logger *logger.Logger) *ProgressManager {
	return &ProgressManager{
		reporters: make(map[string]*ProgressReporter),
		logger:    logger,
	}
}

// CreateReporter creates a new progress reporter
func (m *ProgressManager) CreateReporter(id, operation string, total int) *ProgressReporter {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	reporter := NewProgressReporter(m.logger, operation, total)
	m.reporters[id] = reporter
	return reporter
}

// GetReporter returns a progress reporter by ID
func (m *ProgressManager) GetReporter(id string) (*ProgressReporter, bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	reporter, exists := m.reporters[id]
	return reporter, exists
}

// RemoveReporter removes a progress reporter by ID
func (m *ProgressManager) RemoveReporter(id string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	delete(m.reporters, id)
}

// GetAllReporters returns all progress reporters
func (m *ProgressManager) GetAllReporters() map[string]*ProgressReporter {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Return a copy to avoid race conditions
	reporters := make(map[string]*ProgressReporter)
	for id, reporter := range m.reporters {
		reporters[id] = reporter
	}
	return reporters
}

// GetOverallProgress returns the overall progress across all reporters
func (m *ProgressManager) GetOverallProgress() (int, int) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	totalCurrent := 0
	totalTotal := 0

	for _, reporter := range m.reporters {
		current, total := reporter.GetProgress()
		totalCurrent += current
		totalTotal += total
	}

	return totalCurrent, totalTotal
}

// CompleteAll completes all progress reporters
func (m *ProgressManager) CompleteAll() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for _, reporter := range m.reporters {
		reporter.Complete()
	}
}

// WaitForAllCompletion waits for all progress reporters to complete
func (m *ProgressManager) WaitForAllCompletion() {
	m.mutex.Lock()
	reporters := make([]*ProgressReporter, 0, len(m.reporters))
	for _, reporter := range m.reporters {
		reporters = append(reporters, reporter)
	}
	m.mutex.Unlock()

	for _, reporter := range reporters {
		reporter.WaitForCompletion()
	}
}
