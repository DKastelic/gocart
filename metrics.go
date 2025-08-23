package main

import (
	"sync"
	"time"
)

// MessageMetrics tracks various message-related performance metrics
type MessageMetrics struct {
	mu sync.RWMutex

	// Round trip time measurements
	pendingMessages    map[int64]time.Time // RequestId -> timestamp when sent
	roundTripTimes     []time.Duration     // Collection of round trip times
	totalRoundTripTime time.Duration       // Sum of all round trip times
	messageCount       int64               // Total number of messages processed

	// Goal to movement timing
	goalReceivedTime     *time.Time      // When the last goal was received
	movementStartTime    *time.Time      // When movement actually started
	goalToMovementDelays []time.Duration // Collection of goal-to-movement delays

	// Message counting for scenarios
	scenarioMessageCount int64 // Messages sent/received during current scenario
	scenarioStartTime    *time.Time
}

// NewMessageMetrics creates a new message metrics tracker
func NewMessageMetrics() *MessageMetrics {
	return &MessageMetrics{
		pendingMessages:      make(map[int64]time.Time),
		roundTripTimes:       make([]time.Duration, 0),
		goalToMovementDelays: make([]time.Duration, 0),
	}
}

// RecordMessageSent records when a message was sent (for round trip time calculation)
func (m *MessageMetrics) RecordMessageSent(requestId int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pendingMessages[requestId] = time.Now()
	m.scenarioMessageCount++
}

// RecordResponseSent records when a response was sent (for message counting)
func (m *MessageMetrics) RecordResponseSent() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scenarioMessageCount++
}

// RecordMessageReceived records when a response was received and calculates round trip time
func (m *MessageMetrics) RecordMessageReceived(requestId int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if sentTime, exists := m.pendingMessages[requestId]; exists {
		roundTripTime := time.Since(sentTime)
		m.roundTripTimes = append(m.roundTripTimes, roundTripTime)
		m.totalRoundTripTime += roundTripTime
		m.messageCount++
		delete(m.pendingMessages, requestId)
	}
}

// RecordGoalReceived records when a goal was received
func (m *MessageMetrics) RecordGoalReceived() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	m.goalReceivedTime = &now
}

// RecordMovementStart records when movement actually started and calculates goal-to-movement delay
func (m *MessageMetrics) RecordMovementStart() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	m.movementStartTime = &now

	if m.goalReceivedTime != nil {
		delay := now.Sub(*m.goalReceivedTime)
		m.goalToMovementDelays = append(m.goalToMovementDelays, delay)
	}
}

// StartScenario resets scenario-specific metrics
func (m *MessageMetrics) StartScenario() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	m.scenarioStartTime = &now
	m.scenarioMessageCount = 0
}

// GetAverageRoundTripTime calculates and returns the average round trip time
func (m *MessageMetrics) GetAverageRoundTripTime() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.messageCount == 0 {
		return 0
	}
	return m.totalRoundTripTime / time.Duration(m.messageCount)
}

// GetAverageGoalToMovementDelay calculates and returns the average goal-to-movement delay
func (m *MessageMetrics) GetAverageGoalToMovementDelay() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.goalToMovementDelays) == 0 {
		return 0
	}

	var total time.Duration
	for _, delay := range m.goalToMovementDelays {
		total += delay
	}
	return total / time.Duration(len(m.goalToMovementDelays))
}

// GetMessageCount returns the total number of completed message round trips
func (m *MessageMetrics) GetMessageCount() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.messageCount
}

// GetScenarioMessageCount returns the number of messages in the current scenario
func (m *MessageMetrics) GetScenarioMessageCount() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.scenarioMessageCount
}

// GetDetailedMetrics returns detailed metrics for reporting
func (m *MessageMetrics) GetDetailedMetrics() MessageMetricsReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return MessageMetricsReport{
		AverageRoundTripTime:      m.GetAverageRoundTripTime(),
		AverageGoalToMovementTime: m.GetAverageGoalToMovementDelay(),
		TotalMessageCount:         m.messageCount,
		ScenarioMessageCount:      m.scenarioMessageCount,
		RoundTripTimeCount:        int64(len(m.roundTripTimes)),
		GoalToMovementCount:       int64(len(m.goalToMovementDelays)),
	}
}

// MessageMetricsReport contains metrics data for reporting
type MessageMetricsReport struct {
	AverageRoundTripTime      time.Duration `json:"averageRoundTripTime"`
	AverageGoalToMovementTime time.Duration `json:"averageGoalToMovementTime"`
	TotalMessageCount         int64         `json:"totalMessageCount"`
	ScenarioMessageCount      int64         `json:"scenarioMessageCount"`
	RoundTripTimeCount        int64         `json:"roundTripTimeCount"`
	GoalToMovementCount       int64         `json:"goalToMovementCount"`
}
