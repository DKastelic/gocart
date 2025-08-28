package main

import (
	"fmt"
	"log"
	"os"
	"time"
)

type State int

const (
	Idle State = iota
	Moving
	Requesting
	Avoiding
	Busy
	Stopping
)

func (s State) String() string {
	switch s {
	case Idle:
		return "Idle"
	case Moving:
		return "Moving"
	case Requesting:
		return "Requesting"
	case Avoiding:
		return "Avoiding"
	case Busy:
		return "Busy"
	case Stopping:
		return "Stopping"
	default:
		return "Unknown"
	}
}

type Side int

const (
	Left Side = iota
	Right
)

type RequestParameters struct {
	Goal        float64
	Request     Request
	RetryTime   time.Time // Time when the request is to be retried
	AcceptState State     // State to transition to if the request is accepted

	// For forwarding responses from border expansion requests
	OriginalRequest          *Request          // The original request that triggered this border expansion
	OriginalTrajectory       *Trajectory       // The original trajectory to update
	OriginalUpdateTrajectory func(*Trajectory) // Function to update trajectory for original request
	OriginalOutgoingResponse chan Response     // Channel to send response for original request

	// For emergency stop confirmation forwarding
	PendingEmergencyStopConfirmation *EmergencyStopConfirmation
}

type EmergencyStopConfirmation struct {
	RequestId        int64
	OutgoingResponse chan Response
}

type Controller struct {
	Cart                  *Cart
	LeftBorderTrajectory  *Trajectory
	RightBorderTrajectory *Trajectory
	CurrentTrajectory     *Trajectory
	State                 State     // Current state of the controller
	GoalTimestamp         int64     // Timestamp of the current goal request
	BusyUntil             time.Time // Time until which the controller is busy

	VelocityPID     *PID
	PositionPID     *PID
	MovementPlanner *MovementPlanner
	safetyMargin    float64 // Safety margin for goal requests

	// Metrics for performance monitoring
	Metrics *MessageMetrics

	IncomingGoalRequest   chan float64  // Channel for incoming goal requests
	IncomingEmergencyStop chan bool     // Channel for emergency stop commands
	GoalCompletionReport  chan bool     // Channel to report goal completion to goal manager
	StopController        chan struct{} // Channel to stop the controller loop

	// Channels for inter-controller communication
	OutgoingRightRequest  chan Request
	IncomingRightRequest  chan Request
	OutgoingRightResponse chan Response
	IncomingRightResponse chan Response
	OutgoingLeftRequest   chan Request
	IncomingLeftRequest   chan Request
	OutgoingLeftResponse  chan Response
	IncomingLeftResponse  chan Response

	// Pending requests indexed by request ID
	PendingRequests map[int64]*RequestParameters

	// Pending emergency stop confirmation to send after our own stop is complete
	PendingEmergencyStopConfirmation *EmergencyStopConfirmation

	// Pending goal to handle after stopping is complete
	pendingGoalAfterStop *float64

	// Logger for this controller
	logger *log.Logger
}

// LogLevel represents the level of logging
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// log writes a log message with the specified level
func (c *Controller) log(level LogLevel, format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	c.logger.Printf("[%s] %s: %s", level.String(), c.Cart.Name, message)
}

// logDebug logs a debug message
func (c *Controller) logDebug(format string, args ...interface{}) {
	c.log(DEBUG, format, args...)
}

// logInfo logs an info message
func (c *Controller) logInfo(format string, args ...interface{}) {
	c.log(INFO, format, args...)
}

// logWarn logs a warning message
func (c *Controller) logWarn(format string, args ...interface{}) {
	c.log(WARN, format, args...)
}

// logError logs an error message
func (c *Controller) logError(format string, args ...interface{}) {
	c.log(ERROR, format, args...)
}

// NewController creates a new controller for a cart
func NewController(cart *Cart, leftBorder, rightBorder float64) *Controller {
	// Initialize trajectories
	movementPlanner := NewMovementPlanner(200, 100, 300)
	leftBorderTrajectory := movementPlanner.GetStationaryTrajectory(leftBorder)
	rightBorderTrajectory := movementPlanner.GetStationaryTrajectory(rightBorder)
	currentTrajectory := movementPlanner.GetStationaryTrajectory(cart.Position)

	return &Controller{
		Cart:                  cart,
		VelocityPID:           NewPID(150, 10, 0, 0.01, 150),
		PositionPID:           NewPID(100, 0, 0, 0.01, 300),
		MovementPlanner:       movementPlanner,
		safetyMargin:          30,
		Metrics:               NewMessageMetrics(), // Initialize metrics tracking
		LeftBorderTrajectory:  leftBorderTrajectory,
		RightBorderTrajectory: rightBorderTrajectory,
		CurrentTrajectory:     currentTrajectory,
		IncomingGoalRequest:   make(chan float64, 10), // Buffered to prevent blocking
		IncomingEmergencyStop: make(chan bool, 10),    // Buffered to prevent blocking
		GoalCompletionReport:  make(chan bool, 10),    // Buffered to prevent blocking
		StopController:        make(chan struct{}),    // Channel to stop the controller
		State:                 Idle,
		PendingRequests:       make(map[int64]*RequestParameters),
		logger:                log.New(os.Stdout, "", log.LstdFlags),
	}
}

// run_controller starts the controller's main loop
func (c *Controller) run_controller() {
	c.logInfo("Starting controller main loop")

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// run PID controllers
			c.runPIDControllers()

			// state machine
			switch c.State {
			case Busy:
				if time.Now().After(c.BusyUntil) {
					c.logInfo("Busy period ended, returning to idle state")
					c.State = Idle
					// Report goal completion when busy period ends
					select {
					case c.GoalCompletionReport <- true:
					default:
						// Channel full, skip reporting to avoid blocking
					}
				}
			case Moving:
				// Check if the cart has reached the goal
				if c.CurrentTrajectory.IsFinished() {
					c.logInfo("Goal reached!")
					c.State = Busy
					c.BusyUntil = time.Now().Add(5000 * time.Millisecond) // Simulate busy state for 5s
				}
			case Avoiding:
				// Check if the cart has reached the goal
				if c.CurrentTrajectory.IsFinished() {
					c.logInfo("Goal reached!")
					c.State = Idle
					// Report goal completion for avoiding maneuvers too
					select {
					case c.GoalCompletionReport <- true:
					default:
						// Channel full, skip reporting to avoid blocking
					}
				}
			case Requesting:
				// check if there are any requests to retry
				c.retryPendingRequests()
			case Idle:
				// check if there are any requests to retry
				c.retryPendingRequests()
			case Stopping:
				// Check if stopping is complete
				if c.CurrentTrajectory.IsFinished() {
					c.logInfo("Stopping completed")

					// Check if there's a pending goal to handle after stopping
					if c.pendingGoalAfterStop != nil {
						goal := *c.pendingGoalAfterStop
						c.pendingGoalAfterStop = nil // Clear the pending goal
						c.logInfo("Processing pending goal after stop: %.2f", goal)
						c.handleGoalRequest(goal, Moving)
					} else {
						c.State = Idle
						// Notify goal manager that we're ready for a new goal (previous goal was interrupted)
						select {
						case c.GoalCompletionReport <- true:
						default:
							// Channel full, skip reporting to avoid blocking
						}
					}
				}
			}

		case <-c.StopController:
			c.logInfo("Controller stop signal received, exiting main loop")
			return

		case goal := <-c.IncomingGoalRequest:
			c.logDebug("Processing goal request: %.2f in state %s", goal, c.State)
			switch c.State {
			case Idle, Requesting:
				c.handleGoalRequest(goal, Moving)
			case Moving, Avoiding, Stopping:
				c.handleGoalRequestDuringMovement(goal)
			default:
				c.logWarn("Ignoring goal request while not idle (state: %s)", c.State)
			}

		case <-c.IncomingEmergencyStop:
			c.logInfo("Emergency stop signal received")
			c.handleEmergencyStop()

		case request := <-c.IncomingRightRequest:
			c.logDebug("Incoming request from right neighbor (ID: %d, Type: %v)", request.RequestId, request.Type)
			c.handleIncomingRequest(request, Right)
		case request := <-c.IncomingLeftRequest:
			c.logDebug("Incoming request from left neighbor (ID: %d, Type: %v)", request.RequestId, request.Type)
			c.handleIncomingRequest(request, Left)

		case response := <-c.IncomingRightResponse:
			c.logDebug("Received response from right neighbor (ID: %d, Type: %v)", response.RequestId, response.Type)
			c.handleResponse(response, Right)
		case response := <-c.IncomingLeftResponse:
			c.logDebug("Received response from left neighbor (ID: %d, Type: %v)", response.RequestId, response.Type)
			c.handleResponse(response, Left)
		}
	}
}

func (c *Controller) retryPendingRequests() {
	for requestId, pendingRequest := range c.PendingRequests {
		if time.Now().After(pendingRequest.RetryTime) {
			c.logDebug("Request %d is ready for retry", requestId)
			// Skip retrying requests that have an original request, as the original request will be retried anyways
			if pendingRequest.OriginalRequest != nil {
				c.logDebug("Skipping retry for request %d as it has an original request", requestId)
				continue
			}

			c.logDebug("Retrying request with ID: %d", pendingRequest.Request.RequestId)

			// Handle retry based on request type
			switch pendingRequest.Request.Type {
			case EMERGENCY_STOP:
				// For emergency stop requests, re-evaluate the stopping conditions and resend if needed
				c.logDebug("Re-evaluating emergency stop conditions for retry")
				// Remove the old entry first
				delete(c.PendingRequests, requestId)
				// Re-run the emergency stop logic which will send new requests if needed
				c.sendEmergencyStopRequestsAndWait()
				return // Exit early since sendEmergencyStopRequestsAndWait will handle everything
			case BORDER_MOVE:
				// Remove the old entry since queueBorderMoveRequest will add a new one
				delete(c.PendingRequests, requestId)
				// For border move requests, use the existing retry logic
				c.queueBorderMoveRequest(
					pendingRequest.Goal,
					pendingRequest.Request.RequestId, // Use the request ID from the original request
					pendingRequest.AcceptState,
					&requestId, // Pass the old request ID for retry
					pendingRequest.OriginalRequest,
					pendingRequest.OriginalUpdateTrajectory,
					pendingRequest.OriginalTrajectory,
					pendingRequest.OriginalOutgoingResponse,
				)
			default:
				c.logError("Unknown request type for retry: %v", pendingRequest.Request.Type)
				delete(c.PendingRequests, requestId)
			}
		}
	}
}

func (c *Controller) runPIDControllers() {
	c.PositionPID.SetSetpoint(c.CurrentTrajectory.GetCurrentPosition())
	control_velocity := c.PositionPID.Update(c.Cart.Position)
	c.VelocityPID.SetSetpoint(control_velocity)
	control_force := c.VelocityPID.Update(c.Cart.Velocity)
	c.Cart.applyForce(control_force)
}

func (c *Controller) handleGoalRequest(goal float64, acceptState State) {
	c.handleGoalRequestWithOriginal(goal, acceptState, nil, nil, nil, nil)
}

func (c *Controller) handleGoalRequestWithOriginal(goal float64, acceptState State, originalRequest *Request, originalUpdateTrajectory func(*Trajectory), originalTrajectory *Trajectory, originalOutgoingResponse chan Response) {
	c.logInfo("Received goal request: %.2f", goal)

	// Record goal received for goal-to-movement timing
	c.Metrics.RecordGoalReceived()

	goalTimestamp := time.Now().UnixNano()

	if c.LeftBorderTrajectory.end+c.safetyMargin < goal && goal < c.RightBorderTrajectory.end-c.safetyMargin {
		c.logDebug("Goal %.2f is within borders [%.2f, %.2f] with safety margin %.2f", goal, c.LeftBorderTrajectory.end, c.RightBorderTrajectory.end, c.safetyMargin)
		c.acceptGoal(goal, goalTimestamp, acceptState)
	} else {
		c.logDebug("Goal %.2f is outside borders, need to expand", goal)
		c.queueBorderMoveRequest(goal, goalTimestamp, acceptState, nil, originalRequest, originalUpdateTrajectory, originalTrajectory, originalOutgoingResponse)
	}
}

func (c *Controller) handleGoalRequestDuringMovement(goal float64) {
	c.logInfo("Received goal request during movement, stopping first: %.2f", goal)

	// Store the new goal to handle after stopping is complete
	c.pendingGoalAfterStop = &goal

	// Check if the pending goal would require border expansion
	// If so, we should notify the relevant neighbor about the upcoming stop
	goalRequiresLeftBorderExpansion := c.LeftBorderTrajectory.end+c.safetyMargin >= goal
	goalRequiresRightBorderExpansion := c.RightBorderTrajectory.end-c.safetyMargin <= goal

	c.logDebug("Goal analysis: leftExpansion=%v, rightExpansion=%v", goalRequiresLeftBorderExpansion, goalRequiresRightBorderExpansion)

	// Use the emergency stop method to properly coordinate with neighbors
	// But first, let neighbors know why we're stopping if it affects their borders
	if goalRequiresLeftBorderExpansion && c.OutgoingLeftRequest != nil {
		c.logDebug("Pending goal %.2f would require left border expansion, notifying left neighbor", goal)
	}
	if goalRequiresRightBorderExpansion && c.OutgoingRightRequest != nil {
		c.logDebug("Pending goal %.2f would require right border expansion, notifying right neighbor", goal)
	}

	c.handleEmergencyStop()
}

func (c *Controller) acceptGoal(goal float64, goalTimestamp int64, acceptState State) {
	c.logInfo("Goal accepted: %.2f", goal)
	c.State = acceptState
	c.GoalTimestamp = goalTimestamp
	// Handle incoming goal request
	c.CurrentTrajectory = c.MovementPlanner.CalculatePointToPointTrajectory(c.CurrentTrajectory.GetCurrentPosition(), goal)

	// Record movement start for goal-to-movement timing
	if acceptState == Moving {
		c.Metrics.RecordMovementStart()
	}
}

func (c *Controller) rejectGoal(goal float64) {
	c.logWarn("Goal permanently rejected: %.2f", goal)
	c.State = Idle
	// Notify goal manager that goal was rejected so it can send a new one
	select {
	case c.GoalCompletionReport <- true:
	default:
		// Channel full, skip reporting to avoid blocking
	}
}

func (c *Controller) postponeGoal(goal float64) {
	c.logDebug("Goal postponed: %.2f", goal)
}

func (c *Controller) queueBorderMoveRequest(goal float64, goalTimestamp int64, acceptState State, oldRequestId *int64, originalRequest *Request, originalUpdateTrajectory func(*Trajectory), originalTrajectory *Trajectory, originalOutgoingResponse chan Response) {
	c.logDebug("Goal out of bounds, queuing border move request: %.2f", goal)
	c.State = Requesting

	// Helper function to handle border move requests
	trySendRequest := func(outgoing chan Request, start, end float64) {
		c.logDebug("Attempting to send border move request: start=%.2f, end=%.2f", start, end)
		if outgoing == nil {
			c.logWarn("No neighbor available for border move request")
			// No neighbor, reject request
			c.rejectGoal(goal)
			// If this was triggered by an original request, reject that request too
			if originalRequest != nil && originalOutgoingResponse != nil {
				c.rejectRequest(originalOutgoingResponse, *originalRequest)
			}
		} else {
			var requestId int64
			if oldRequestId != nil {
				requestId = *oldRequestId
			} else if originalRequest != nil {
				requestId = originalRequest.RequestId // Inherit ID from original request for forwarded requests
			} else {
				requestId = goalTimestamp // Unique request ID based on current time
			}
			c.logDebug("Sending border move request with ID %d", requestId)
			request := Request{
				RequestId:           requestId,
				Type:                BORDER_MOVE,
				ProposedBorderStart: start,
				ProposedBorderEnd:   end,
			}
			requestParameters := RequestParameters{
				Goal:                     goal,
				Request:                  request,
				RetryTime:                time.Now().Add(1000 * time.Millisecond),
				AcceptState:              acceptState, // State to transition to if the request is accepted
				OriginalRequest:          originalRequest,
				OriginalUpdateTrajectory: originalUpdateTrajectory,
				OriginalTrajectory:       originalTrajectory,
				OriginalOutgoingResponse: originalOutgoingResponse,
			}
			c.PendingRequests[requestId] = &requestParameters
			// Record message sent for round trip time measurement
			c.Metrics.RecordMessageSent(requestId)
			// Send the request to the neighbor controller
			outgoing <- request
		}
	}

	if c.LeftBorderTrajectory.end+c.safetyMargin >= goal {
		c.logDebug("Goal requires left border expansion")
		trySendRequest(
			c.OutgoingLeftRequest,
			c.LeftBorderTrajectory.end,
			goal-1.01*c.safetyMargin,
		)
	} else if c.RightBorderTrajectory.end-c.safetyMargin <= goal {
		c.logDebug("Goal requires right border expansion")
		trySendRequest(
			c.OutgoingRightRequest,
			c.RightBorderTrajectory.end,
			goal+1.01*c.safetyMargin,
		)
	} else {
		// unreachable
		c.logError("Goal inside bounds but still got to processing border move request")
	}
}

func (c *Controller) handleResponse(response Response, side Side) {
	c.logDebug("Handling response ID %d of type %v from %s neighbor", response.RequestId, response.Type, map[Side]string{Left: "left", Right: "right"}[side])

	if c.State != Requesting {
		c.logWarn("Ignoring response in state %s", c.State)
		return
	}

	// Record message received for round trip time measurement
	c.Metrics.RecordMessageReceived(response.RequestId)

	requestParams, exists := c.PendingRequests[response.RequestId]
	if !exists {
		c.logWarn("Received response for unknown or old request, ignoring")
		return
	}

	// Remove the request from pending requests after handling
	delete(c.PendingRequests, response.RequestId)

	switch response.Type {
	case ACCEPT:
		c.logDebug("Processing ACCEPT response")
		c.handleAcceptResponse(*requestParams, side)
	case REJECT:
		c.logDebug("Processing REJECT response")
		c.handleRejectResponse(*requestParams)
	case WAIT:
		c.logDebug("Processing WAIT response")
		c.handleWaitResponse(*requestParams)
	case STOP_CONFIRM:
		c.logDebug("Processing STOP_CONFIRM response")
		c.handleStopConfirmResponse()
	default:
		c.logWarn("Unknown response received")
	}
}

func (c *Controller) handleAcceptResponse(requestParams RequestParameters, side Side) {
	c.logInfo("Border move request accepted")
	// Update the border trajectory based on the accepted request
	if side == Left {
		c.logDebug("Updating left border trajectory to %.2f", requestParams.Request.ProposedBorderEnd)
		c.LeftBorderTrajectory = c.MovementPlanner.CalculatePointToPointTrajectory(
			c.LeftBorderTrajectory.GetCurrentPosition(),
			requestParams.Request.ProposedBorderEnd,
		)
	} else {
		c.logDebug("Updating right border trajectory to %.2f", requestParams.Request.ProposedBorderEnd)
		c.RightBorderTrajectory = c.MovementPlanner.CalculatePointToPointTrajectory(
			c.RightBorderTrajectory.GetCurrentPosition(),
			requestParams.Request.ProposedBorderEnd,
		)
	}

	// Accept the goal and start moving towards it
	c.acceptGoal(requestParams.Goal, requestParams.Request.RequestId, requestParams.AcceptState)

	// If this was triggered by an original request, accept that request too
	if requestParams.OriginalRequest != nil && requestParams.OriginalUpdateTrajectory != nil && requestParams.OriginalOutgoingResponse != nil {
		c.logDebug("Forwarding accept to original request ID %d", requestParams.OriginalRequest.RequestId)
		c.acceptRequest(requestParams.OriginalUpdateTrajectory, requestParams.OriginalTrajectory, requestParams.OriginalOutgoingResponse, *requestParams.OriginalRequest)
	}
}

func (c *Controller) handleRejectResponse(requestParams RequestParameters) {
	c.logWarn("Border move request rejected")
	// Reject the goal and stop moving
	c.rejectGoal(requestParams.Goal)

	// If this was triggered by an original request, reject that request too
	if requestParams.OriginalRequest != nil && requestParams.OriginalOutgoingResponse != nil {
		c.logDebug("Forwarding reject to original request ID %d", requestParams.OriginalRequest.RequestId)
		c.rejectRequest(requestParams.OriginalOutgoingResponse, *requestParams.OriginalRequest)
	}
}

func (c *Controller) handleWaitResponse(requestParams RequestParameters) {
	c.logDebug("Border move request waiting for response")
	requestParams.RetryTime = time.Now().Add(1000 * time.Millisecond) // Retry after 1000ms
	c.PendingRequests[requestParams.Request.RequestId] = &requestParams
	c.postponeGoal(requestParams.Goal)

	// If this was triggered by an original request, postpone that request too
	if requestParams.OriginalRequest != nil && requestParams.OriginalUpdateTrajectory != nil && requestParams.OriginalOutgoingResponse != nil {
		c.logDebug("Forwarding postpone to original request ID %d", requestParams.OriginalRequest.RequestId)
		c.postponeRequest(requestParams.OriginalOutgoingResponse, *requestParams.OriginalRequest)
	}
}

func (c *Controller) handleStopConfirmResponse() {
	c.logInfo("Emergency stop confirmed by neighbor")

	// Now execute our own emergency stop since we got confirmation
	c.executeEmergencyStop()
}

func (c *Controller) handleIncomingRequest(request Request, side Side) {
	sideStr := map[Side]string{Left: "left", Right: "right"}[side]
	c.logDebug("Processing incoming %v request from %s neighbor (ID: %d)", request.Type, sideStr, request.RequestId)

	switch request.Type {
	case BORDER_MOVE:
		c.handleIncomingBorderMoveRequest(request, side)
	case EMERGENCY_STOP:
		c.handleIncomingEmergencyStopRequest(request, side)
	default:
		c.logWarn("Unknown request type received")
	}
}

func (c *Controller) handleIncomingBorderMoveRequest(request Request, side Side) {
	sideStr := map[Side]string{Left: "left", Right: "right"}[side]
	c.logDebug("Processing border move request from %s neighbor (ID: %d, border end: %.2f)", sideStr, request.RequestId, request.ProposedBorderEnd)

	// Check if we have a conflicting pending request to the same neighbor OR if our current goal conflicts
	hasConflictingRequest := false
	shouldDeferToNeighbor := false

	// First check pending requests
	for _, pendingRequest := range c.PendingRequests {
		if pendingRequest.Request.Type == BORDER_MOVE {
			// Check if we're trying to expand toward the same neighbor
			if side == Left && c.LeftBorderTrajectory.end+c.safetyMargin >= pendingRequest.Goal {
				hasConflictingRequest = true
				// Use request ID as tie-breaker: higher ID gets priority
				shouldDeferToNeighbor = request.RequestId > pendingRequest.Request.RequestId
				c.logDebug("Conflicting left border expansion detected: their request %d vs our pending %d (defer: %v)",
					request.RequestId, pendingRequest.Request.RequestId, shouldDeferToNeighbor)
				break
			} else if side == Right && c.RightBorderTrajectory.end-c.safetyMargin <= pendingRequest.Goal {
				hasConflictingRequest = true
				// Use request ID as tie-breaker: higher ID gets priority
				shouldDeferToNeighbor = request.RequestId > pendingRequest.Request.RequestId
				c.logDebug("Conflicting right border expansion detected: their request %d vs our pending %d (defer: %v)",
					request.RequestId, pendingRequest.Request.RequestId, shouldDeferToNeighbor)
				break
			}
		}
	}

	// Check if we're moving or avoiding toward the same neighbor
	if c.State == Moving || c.State == Avoiding {
		if side == Left && c.CurrentTrajectory.end-c.safetyMargin < request.ProposedBorderEnd {
			hasConflictingRequest = true
			shouldDeferToNeighbor = request.RequestId > c.GoalTimestamp
			c.logDebug("Conflicting left border expansion detected: their request %d vs our goal %d (defer: %v)",
				request.RequestId, c.GoalTimestamp, shouldDeferToNeighbor)
		} else if side == Right && c.CurrentTrajectory.end+c.safetyMargin > request.ProposedBorderEnd {
			hasConflictingRequest = true
			shouldDeferToNeighbor = request.RequestId > c.GoalTimestamp
			c.logDebug("Conflicting right border expansion detected: their request %d vs our goal %d (defer: %v)",
				request.RequestId, c.GoalTimestamp, shouldDeferToNeighbor)
		}
	}

	if side == Left {
		// Accept if the proposed border doesn't interfere with our current position + safety margin
		// AND we don't have a conflicting request OR we should defer to them
		// AND it doesn't create unsafe overlap
		acceptImmediately := request.ProposedBorderEnd < c.CurrentTrajectory.end-c.safetyMargin && (!hasConflictingRequest || shouldDeferToNeighbor)
		c.logDebug("Left border request: acceptImmediately=%v (proposed: %.2f, end: %.2f, safety: %.2f, conflict: %v, defer: %v)",
			acceptImmediately, request.ProposedBorderEnd, c.CurrentTrajectory.end, c.safetyMargin, hasConflictingRequest, shouldDeferToNeighbor)

		// If we have a conflict and should defer, we need to stop our current movement first
		if hasConflictingRequest && shouldDeferToNeighbor && (c.State == Moving || c.State == Avoiding) {
			c.logDebug("Deferring to neighbor - stopping current movement to give way")
			// Store the border request to handle after stopping
			// avoidanceGoal := request.ProposedBorderEnd + 1.01*c.safetyMargin
			c.handleEmergencyStop()
			// Postpone the request until we can properly handle it after stopping
			c.postponeRequest(c.OutgoingLeftResponse, request)
		} else if hasConflictingRequest && !shouldDeferToNeighbor {
			c.logDebug("Not deferring to neighbor - prioritising our request")
			c.rejectRequest(c.OutgoingLeftResponse, request)
		} else {
			c.handleBorderMove(
				acceptImmediately,
				func(t *Trajectory) { c.LeftBorderTrajectory = t },
				c.LeftBorderTrajectory,
				c.OutgoingLeftResponse,
				request,
				side,
			)
		}
	} else {
		// Accept if the proposed border doesn't interfere with our current position + safety margin
		// AND we don't have a conflicting request OR we should defer to them
		// AND it doesn't create unsafe overlap
		acceptImmediately := request.ProposedBorderEnd > c.CurrentTrajectory.end+c.safetyMargin && (!hasConflictingRequest || shouldDeferToNeighbor)
		c.logDebug("Right border request: acceptImmediately=%v (proposed: %.2f, end: %.2f, safety: %.2f, conflict: %v, defer: %v)",
			acceptImmediately, request.ProposedBorderEnd, c.CurrentTrajectory.end, c.safetyMargin, hasConflictingRequest, shouldDeferToNeighbor)

		// If we have a conflict and should defer, we need to stop our current movement first
		if hasConflictingRequest && shouldDeferToNeighbor && (c.State == Moving || c.State == Avoiding) {
			c.logDebug("Deferring to neighbor - stopping current movement to give way")
			// Store the border request to handle after stopping
			// avoidanceGoal := request.ProposedBorderEnd - 1.01*c.safetyMargin
			c.handleEmergencyStop()
			// Postpone the request until we can properly handle it after stopping
			c.postponeRequest(c.OutgoingRightResponse, request)
		} else if hasConflictingRequest && !shouldDeferToNeighbor {
			c.logDebug("Not deferring to neighbor - prioritising our request")
			c.postponeRequest(c.OutgoingRightResponse, request)
		} else {
			c.handleBorderMove(
				acceptImmediately,
				func(t *Trajectory) { c.RightBorderTrajectory = t },
				c.RightBorderTrajectory,
				c.OutgoingRightResponse,
				request,
				side,
			)
		}
	}
}

func (c *Controller) handleIncomingEmergencyStopRequest(request Request, side Side) {
	sideStr := map[Side]string{Left: "left", Right: "right"}[side]
	c.logInfo("Processing emergency stop request from %s neighbor (ID: %d)", sideStr, request.RequestId)

	// Store the confirmation details to send after our emergency stop is complete
	var outgoingResponse chan Response
	if side == Left {
		outgoingResponse = c.OutgoingLeftResponse
	} else {
		outgoingResponse = c.OutgoingRightResponse
	}

	if outgoingResponse != nil {
		c.PendingEmergencyStopConfirmation = &EmergencyStopConfirmation{
			RequestId:        request.RequestId,
			OutgoingResponse: outgoingResponse,
		}
	}

	// When we receive an emergency stop request, we should also consider if this might
	// be due to a goal that would require further border expansion beyond us.
	// We can't know the exact goal, but we can anticipate that if the neighbor is stopping
	// and will need our border, they might also need borders beyond us.

	// Consider the likely scenario: if left neighbor is stopping, they might have a goal
	// that goes through our territory and potentially beyond to the right.
	// If right neighbor is stopping, they might have a goal that goes left through us and beyond.
	anticipateRightExpansion := false
	anticipateLeftExpansion := false

	if side == Left {
		// Left neighbor stopping might need to expand right through us and potentially further right
		anticipateRightExpansion = true
		c.logDebug("Left neighbor stopping - anticipating potential right expansion needs")
	} else {
		// Right neighbor stopping might need to expand left through us and potentially further left
		anticipateLeftExpansion = true
		c.logDebug("Right neighbor stopping - anticipating potential left expansion needs")
	}

	// Store the anticipated expansion needs for use in sendEmergencyStopRequestsAndWait
	if anticipateLeftExpansion {
		// Temporarily simulate a goal that would require left expansion for coordination purposes
		tempGoal := c.LeftBorderTrajectory.end - c.safetyMargin - 50 // Goal that would need left expansion
		c.pendingGoalAfterStop = &tempGoal
	} else if anticipateRightExpansion {
		// Temporarily simulate a goal that would require right expansion for coordination purposes
		tempGoal := c.RightBorderTrajectory.end + c.safetyMargin + 50 // Goal that would need right expansion
		c.pendingGoalAfterStop = &tempGoal
	}

	// Perform our own emergency stop (which will check borders and send requests if needed)
	// The confirmation will be sent when executeEmergencyStop() is called
	c.handleEmergencyStop()

	// Clear the temporary goal since it was just for coordination
	if anticipateLeftExpansion || anticipateRightExpansion {
		c.pendingGoalAfterStop = nil
	}
}

func (c *Controller) handleBorderMove(
	acceptImmediately bool,
	updateTrajectory func(*Trajectory),
	originalTrajectory *Trajectory,
	outgoingResponse chan Response,
	request Request,
	side Side,
) {
	c.logDebug("Handling border move request ID %d: acceptImmediately=%v", request.RequestId, acceptImmediately)
	if acceptImmediately {
		c.acceptRequest(updateTrajectory, originalTrajectory, outgoingResponse, request)
	} else {
		c.tryToGiveWay(updateTrajectory, originalTrajectory, outgoingResponse, request, side)
	}
}

func (c *Controller) acceptRequest(updateTrajectory func(*Trajectory), originalTrajectory *Trajectory, outgoingResponse chan Response, request Request) {
	c.logDebug("Accepting border move request ID %d (border end: %.2f)", request.RequestId, request.ProposedBorderEnd)
	trajectory := c.MovementPlanner.CalculatePointToPointTrajectory(
		originalTrajectory.GetCurrentPosition(),
		request.ProposedBorderEnd,
	)
	updateTrajectory(trajectory)
	// Record response sent for message counting
	c.Metrics.RecordResponseSent()
	outgoingResponse <- Response{
		RequestId: request.RequestId,
		Type:      ACCEPT,
	}
}

func (c *Controller) rejectRequest(outgoingResponse chan Response, request Request) {
	c.logDebug("Rejecting border move request ID %d", request.RequestId)
	// Record response sent for message counting
	c.Metrics.RecordResponseSent()
	outgoingResponse <- Response{
		RequestId: request.RequestId,
		Type:      REJECT,
	}
}

func (c *Controller) postponeRequest(outgoingResponse chan Response, request Request) {
	c.logDebug("Postponing border move request ID %d", request.RequestId)
	// Record response sent for message counting
	c.Metrics.RecordResponseSent()
	outgoingResponse <- Response{
		RequestId: request.RequestId,
		Type:      WAIT,
	}
}

func (c *Controller) handleEmergencyStop() {
	c.logInfo("Emergency stop initiated!")

	// Send emergency stop requests to neighbors and wait for confirmations
	c.sendEmergencyStopRequestsAndWait()
}

func (c *Controller) sendEmergencyStopRequestsAndWait() {
	c.logDebug("Evaluating emergency stop conditions")

	// Calculate where we would stop if we emergency brake now
	hypotheticalStopTrajectory := c.MovementPlanner.CalculateStoppingTrajectory(
		c.CurrentTrajectory,
	)

	// Get the final position after stopping
	finalStopPosition := hypotheticalStopTrajectory.end
	c.logDebug("Hypothetical stop position: %.2f", finalStopPosition)

	// Check if the stop position would violate any border (including safety margin)
	leftBorderEnd := c.LeftBorderTrajectory.end
	rightBorderEnd := c.RightBorderTrajectory.end

	violatesLeftBorder := finalStopPosition < leftBorderEnd+c.safetyMargin
	violatesRightBorder := finalStopPosition > rightBorderEnd-c.safetyMargin

	c.logDebug("Border violation check: left=%v, right=%v (borders: [%.2f, %.2f], safety: %.2f)",
		violatesLeftBorder, violatesRightBorder, leftBorderEnd, rightBorderEnd, c.safetyMargin)

	// Also check if we have a pending goal that would require border expansion
	pendingGoalRequiresLeftExpansion := false
	pendingGoalRequiresRightExpansion := false
	if c.pendingGoalAfterStop != nil {
		goal := *c.pendingGoalAfterStop
		pendingGoalRequiresLeftExpansion = leftBorderEnd+c.safetyMargin >= goal
		pendingGoalRequiresRightExpansion = rightBorderEnd-c.safetyMargin <= goal
		c.logDebug("Pending goal %.2f expansion requirements: left=%v, right=%v", goal, pendingGoalRequiresLeftExpansion, pendingGoalRequiresRightExpansion)
	}

	// Only send requests to neighbors whose borders would be violated OR who we'll need for pending goal
	needsConfirmation := false
	c.logDebug("Checking if emergency stop confirmation needed")

	if (violatesLeftBorder || pendingGoalRequiresLeftExpansion) && c.OutgoingLeftRequest != nil {
		c.logDebug("Sending emergency stop request to left neighbor")
		requestId := time.Now().UnixNano()
		emergencyStopRequest := Request{
			RequestId: requestId,
			Type:      EMERGENCY_STOP,
		}

		// Store this as a pending request to track confirmations
		requestParams := RequestParameters{
			Request:     emergencyStopRequest,
			RetryTime:   time.Now().Add(1000 * time.Millisecond), // Retry if no response
			AcceptState: Stopping,                                // State to transition to when confirmed
		}
		c.PendingRequests[requestId] = &requestParams
		// Record message sent for round trip time measurement
		c.Metrics.RecordMessageSent(requestId)
		c.OutgoingLeftRequest <- emergencyStopRequest
		needsConfirmation = true

		if violatesLeftBorder {
			c.logWarn("Stop position %.2f would violate left border at %.2f - requesting confirmation", finalStopPosition, leftBorderEnd)
		}
		if pendingGoalRequiresLeftExpansion {
			c.logDebug("Pending goal %.2f will require left border expansion - requesting stop confirmation", *c.pendingGoalAfterStop)
		}
	}

	if (violatesRightBorder || pendingGoalRequiresRightExpansion) && c.OutgoingRightRequest != nil {
		c.logDebug("Sending emergency stop request to right neighbor")
		requestId := time.Now().UnixNano()
		emergencyStopRequest := Request{
			RequestId: requestId,
			Type:      EMERGENCY_STOP,
		}

		// Store this as a pending request to track confirmations
		requestParams := RequestParameters{
			Request:     emergencyStopRequest,
			RetryTime:   time.Now().Add(1000 * time.Millisecond), // Retry if no response
			AcceptState: Stopping,                                // State to transition to when confirmed
		}
		c.PendingRequests[requestId] = &requestParams
		// Record message sent for round trip time measurement
		c.Metrics.RecordMessageSent(requestId)
		c.OutgoingRightRequest <- emergencyStopRequest
		needsConfirmation = true

		if violatesRightBorder {
			c.logWarn("Stop position %.2f would violate right border at %.2f - requesting confirmation", finalStopPosition, rightBorderEnd)
		}
		if pendingGoalRequiresRightExpansion {
			c.logDebug("Pending goal %.2f will require right border expansion - requesting stop confirmation", *c.pendingGoalAfterStop)
		}
	}

	if needsConfirmation {
		// Wait for confirmations before stopping
		c.State = Requesting
		c.logDebug("Waiting for emergency stop confirmation(s)")
	} else {
		// No border violations, can stop immediately
		c.logInfo("Stop position %.2f within borders [%.2f, %.2f] - stopping immediately", finalStopPosition, leftBorderEnd, rightBorderEnd)
		c.executeEmergencyStop()
	}
}

func (c *Controller) executeEmergencyStop() {
	c.logInfo("Executing emergency stop now!")

	// Calculate where we would stop if we emergency brake now
	hypotheticalStopTrajectory := c.MovementPlanner.CalculateStoppingTrajectory(
		c.CurrentTrajectory,
	)

	// Get the final position after stopping
	finalStopPosition := hypotheticalStopTrajectory.end

	// Check if the stop position would violate any border (including safety margin)
	leftBorderEnd := c.LeftBorderTrajectory.end
	rightBorderEnd := c.RightBorderTrajectory.end

	violatesLeftBorder := finalStopPosition < leftBorderEnd+c.safetyMargin
	violatesRightBorder := finalStopPosition > rightBorderEnd-c.safetyMargin

	// Check if we have pending emergency stop confirmation (meaning neighbor is stopping)
	neighborStoppingLeft := c.PendingEmergencyStopConfirmation != nil && c.PendingEmergencyStopConfirmation.OutgoingResponse == c.OutgoingLeftResponse
	neighborStoppingRight := c.PendingEmergencyStopConfirmation != nil && c.PendingEmergencyStopConfirmation.OutgoingResponse == c.OutgoingRightResponse

	// Transition to stopping state and stop the cart
	c.State = Stopping
	c.CurrentTrajectory = c.MovementPlanner.CalculateStoppingTrajectory(
		c.CurrentTrajectory,
	)

	// Stop border movements if:
	// 1. Our stop position would violate them, OR
	// 2. We know the neighbor controlling that border is stopping
	if (violatesLeftBorder || neighborStoppingLeft) && !c.LeftBorderTrajectory.IsFinished() {
		leftStopTrajectory := c.MovementPlanner.CalculateStoppingTrajectory(
			c.LeftBorderTrajectory,
		)
		c.LeftBorderTrajectory = leftStopTrajectory
		if neighborStoppingLeft {
			c.logDebug("Stopping left border movement because left neighbor is stopping")
		}
	}

	if (violatesRightBorder || neighborStoppingRight) && !c.RightBorderTrajectory.IsFinished() {
		rightStopTrajectory := c.MovementPlanner.CalculateStoppingTrajectory(
			c.RightBorderTrajectory,
		)
		c.RightBorderTrajectory = rightStopTrajectory
		if neighborStoppingRight {
			c.logDebug("Stopping right border movement because right neighbor is stopping")
		}
	}

	// Clear all pending requests except emergency stop confirmations
	newPendingRequests := make(map[int64]*RequestParameters)
	for id, params := range c.PendingRequests {
		if params.Request.Type == EMERGENCY_STOP {
			newPendingRequests[id] = params
		}
	}
	c.PendingRequests = newPendingRequests

	// Send any pending emergency stop confirmation now that our stop is complete
	if c.PendingEmergencyStopConfirmation != nil {
		// Record response sent for message counting
		c.Metrics.RecordResponseSent()
		c.PendingEmergencyStopConfirmation.OutgoingResponse <- Response{
			RequestId: c.PendingEmergencyStopConfirmation.RequestId,
			Type:      STOP_CONFIRM,
		}
		c.logDebug("Sent emergency stop confirmation after completing our own stop")
		c.PendingEmergencyStopConfirmation = nil
	}
}

func (c *Controller) tryToGiveWay(updateTrajectory func(*Trajectory), originalTrajectory *Trajectory, outgoingResponse chan Response, request Request, side Side) {
	sideStr := map[Side]string{Left: "left", Right: "right"}[side]
	c.logDebug("Attempting to give way to %s neighbor's request ID %d", sideStr, request.RequestId)

	avoidanceGoal := 0.0
	if side == Left {
		avoidanceGoal = request.ProposedBorderEnd + 1.01*c.safetyMargin
	} else {
		avoidanceGoal = request.ProposedBorderEnd - 1.01*c.safetyMargin
	}
	c.logDebug("Calculated avoidance goal: %.2f", avoidanceGoal)

	if c.State == Idle || c.State == Requesting {
		c.logDebug("In compatible state (%s) for giving way", c.State)

		// Check if the avoidance goal is within current borders
		canAvoidImmediately := c.LeftBorderTrajectory.end+c.safetyMargin < avoidanceGoal && avoidanceGoal < c.RightBorderTrajectory.end-c.safetyMargin
		c.logDebug("Can avoid immediately: %v (borders: [%.2f, %.2f], avoidance: %.2f)", canAvoidImmediately, c.LeftBorderTrajectory.end, c.RightBorderTrajectory.end, avoidanceGoal)

		if canAvoidImmediately {
			c.logDebug("Accepting request - avoidance maneuver is within borders")
			// Avoidance maneuver is immediately successful, accept the original request
			c.handleGoalRequest(avoidanceGoal, Avoiding)
			c.acceptRequest(updateTrajectory, originalTrajectory, outgoingResponse, request)
		} else {
			c.logDebug("Need border expansion for avoidance - forwarding request")
			// Avoidance maneuver requires border expansion, forward the response from that process
			c.handleGoalRequestWithOriginal(avoidanceGoal, Avoiding, &request, updateTrajectory, originalTrajectory, outgoingResponse)
		}
	} else {
		c.logDebug("Cannot give way in current state (%s), postponing request", c.State)
		c.postponeRequest(outgoingResponse, request)
	}
}
