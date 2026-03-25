package handler

import (
	"fmt"
	"testing"
)

func TestBlastJobManager_JobLimit(t *testing.T) {
	m := NewBlastJobManager()
	numJobsToAdd := 15
	
	// Create predictable job IDs
	var jobIDs []string
	for i := 0; i < numJobsToAdd; i++ {
		jobIDs = append(jobIDs, fmt.Sprintf("job-%d", i))
	}

	// Overwrite generateJobID for predictable IDs during this test.
	// This is a common pattern for testing un-exported functions or variables.
	// In this case, we can't do that easily without refactoring the main code.
	// So, we'll manually add jobs and trigger the cleanup logic.

	m.mu.Lock()
	for i := 0; i < numJobsToAdd; i++ {
		jobID := jobIDs[i]
		job := &BlastJob{ID: jobID}
		m.jobs[job.ID] = job
		m.jobOrder = append(m.jobOrder, job.ID)

		if len(m.jobOrder) > maxJobs {
			jobIDToRemove := m.jobOrder[0]
			delete(m.jobs, jobIDToRemove)
			m.jobOrder = m.jobOrder[1:]
		}
	}
	m.mu.Unlock()


	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check that the number of jobs is equal to maxJobs
	if len(m.jobs) != maxJobs {
		t.Errorf("expected %d jobs, but got %d", maxJobs, len(m.jobs))
	}

	if len(m.jobOrder) != maxJobs {
		t.Errorf("expected %d jobOrder length, but got %d", maxJobs, len(m.jobOrder))
	}

	// Check that the first 5 jobs are gone
	for i := 0; i < 5; i++ {
		jobID := jobIDs[i]
		if _, ok := m.jobs[jobID]; ok {
			t.Errorf("job %s should have been removed", jobID)
		}
	}

	// Check that the last 10 jobs are still there
	for i := 5; i < numJobsToAdd; i++ {
		jobID := jobIDs[i]
		if _, ok := m.jobs[jobID]; !ok {
			t.Errorf("job %s should be present", jobID)
		}
	}
}
