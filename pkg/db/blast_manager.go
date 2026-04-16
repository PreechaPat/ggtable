package db

import (
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"sync"
	"time"
)

// BlastJobStatus represents the lifecycle of a BLAST request.
type BlastJobStatus string

const (
	BlastJobQueued    BlastJobStatus = "queued"
	BlastJobRunning   BlastJobStatus = "running"
	BlastJobCompleted BlastJobStatus = "completed"
	BlastJobFailed    BlastJobStatus = "failed"
)

// BlastJob keeps track of the BLAST execution state while the command runs.
type BlastJob struct {
	ID        string
	BlastType string
	Status    BlastJobStatus
	Result    string
	Error     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

const maxJobs = 3

// BlastManager stores BLAST job states indexed by job ID.
type BlastManager struct {
	mu       sync.RWMutex
	jobs     map[string]*BlastJob
	jobOrder []string
}

// NewBlastManager constructs a job manager.
func NewBlastManager() *BlastManager {
	return &BlastManager{
		jobs:     make(map[string]*BlastJob),
		jobOrder: make([]string, 0, maxJobs+1),
	}
}

// NewJob registers a queued job for the provided BLAST type and cleans up old jobs.
func (m *BlastManager) NewJob(blastType string) *BlastJob {
	job := &BlastJob{
		ID:        generateJobID(),
		BlastType: blastType,
		Status:    BlastJobQueued,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.jobs[job.ID] = job
	m.jobOrder = append(m.jobOrder, job.ID)

	if len(m.jobOrder) > maxJobs {
		jobIDToRemove := m.jobOrder[0]
		delete(m.jobs, jobIDToRemove)
		m.jobOrder = m.jobOrder[1:]
	}

	return job
}

// SetRunning marks the job as running.
func (m *BlastManager) SetRunning(jobID string) {
	m.updateJob(jobID, func(job *BlastJob) {
		job.Status = BlastJobRunning
	})
}

// CompleteJob stores the BLAST output and marks the job complete.
func (m *BlastManager) CompleteJob(jobID string, result string) {
	m.updateJob(jobID, func(job *BlastJob) {
		job.Status = BlastJobCompleted
		job.Result = result
	})
}

// FailJob records a failure and attaches a user-facing error message.
func (m *BlastManager) FailJob(jobID string, err error) {
	m.updateJob(jobID, func(job *BlastJob) {
		job.Status = BlastJobFailed
		job.Error = err.Error()
	})
}

// GetJob fetches a job by ID.
func (m *BlastManager) GetJob(jobID string) (*BlastJob, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	job, ok := m.jobs[jobID]
	return job, ok
}

func (m *BlastManager) updateJob(jobID string, update func(job *BlastJob)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.jobs[jobID]
	if !ok {
		return
	}

	update(job)
	job.UpdatedAt = time.Now()
}

func generateJobID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err == nil {
		return hex.EncodeToString(buf[:])
	}
	return strconv.FormatInt(time.Now().UnixNano(), 16)
}
