package main

import (
	"sync"
	"time"
)

// DecisionTracker maintains race-safe bounded recent-decision history and summary counters.
type DecisionTracker struct {
	mu              sync.Mutex
	maxSize         int
	records         []DecisionRecord
	head            int
	totalDecisions  int64
	liveDecisions   int64
	normalDecisions int64
	selectedCount   int64
	delegatedCount  int64
	rejectedCount   int64
}

// NewDecisionTracker creates a tracker with the given maximum history size.
func NewDecisionTracker(maxSize int) *DecisionTracker {
	if maxSize <= 0 {
		maxSize = MaxHistorySize
	}
	return &DecisionTracker{
		maxSize: maxSize,
		records: make([]DecisionRecord, 0, maxSize),
	}
}

// Record appends a sanitized decision, dropping the oldest record at capacity.
func (dt *DecisionTracker) Record(classification string, outcome DecisionOutcome, candidateCount, matchingCount int, reason string) {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	dt.totalDecisions++
	if classification == "live" {
		dt.liveDecisions++
	} else {
		dt.normalDecisions++
	}

	switch outcome {
	case OutcomeSelected:
		dt.selectedCount++
	case OutcomeDelegated:
		dt.delegatedCount++
	case OutcomeRejected:
		dt.rejectedCount++
	}

	rec := DecisionRecord{
		ID:             dt.totalDecisions,
		Timestamp:      time.Now().UTC().Format(time.RFC3339Nano),
		Classification: classification,
		Outcome:        outcome,
		CandidateCount: candidateCount,
		MatchingCount:  matchingCount,
		Reason:         reason,
	}

	if len(dt.records) < dt.maxSize {
		dt.records = append(dt.records, rec)
	} else {
		dt.records[dt.head] = rec
		dt.head = (dt.head + 1) % dt.maxSize
	}
}

// Snapshot returns recent decisions in reverse chronological order and counters.
func (dt *DecisionTracker) Snapshot() (records []DecisionRecord, total, live, normal, selected, delegated, rejected int64) {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	n := len(dt.records)
	ordered := make([]DecisionRecord, 0, n)
	if n < dt.maxSize {
		for i := n - 1; i >= 0; i-- {
			ordered = append(ordered, dt.records[i])
		}
	} else {
		for i := 0; i < n; i++ {
			idx := (dt.head - 1 - i + n) % n
			ordered = append(ordered, dt.records[idx])
		}
	}

	return ordered, dt.totalDecisions, dt.liveDecisions, dt.normalDecisions, dt.selectedCount, dt.delegatedCount, dt.rejectedCount
}
