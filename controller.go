package main

import (
	"fmt"
	"time"
)

type State int

const (
	Idle State = iota
	Moving
	Requesting
	Avoiding
	Busy
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
	OriginalUpdateTrajectory func(*Trajectory) // Function to update trajectory for original request
	OriginalOutgoingResponse chan Response     // Channel to send response for original request
}

type Controller struct {
	Cart                  *Cart
	LeftBorderTrajectory  *Trajectory
	RightBorderTrajectory *Trajectory
	CurrentTrajectory     *Trajectory
	State                 State     // Current state of the controller
	BusyUntil             time.Time // Time until which the controller is busy

	VelocityPID     *PID
	PositionPID     *PID
	MovementPlanner *MovementPlanner
	safetyMargin    float64 // Safety margin for goal requests

	IncomingGoalRequest chan float64 // Channel for incoming goal requests

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
}

// NewController creates a new controller for a cart
func NewController(cart *Cart, leftBorder, rightBorder float64) *Controller {
	// Initialize trajectories
	movementPlanner := NewMovementPlanner(200, 100, 300)
	leftBorderTrajectory := movementPlanner.CalculateTrajectory(leftBorder, leftBorder)
	rightBorderTrajectory := movementPlanner.CalculateTrajectory(rightBorder, rightBorder)
	currentTrajectory := movementPlanner.CalculateTrajectory(cart.Position, cart.Position)

	return &Controller{
		Cart:                  cart,
		VelocityPID:           NewPID(150, 0, 0, 0.01),
		PositionPID:           NewPID(100, 0, 0, 0.01),
		MovementPlanner:       movementPlanner,
		safetyMargin:          30,
		LeftBorderTrajectory:  &leftBorderTrajectory,
		RightBorderTrajectory: &rightBorderTrajectory,
		CurrentTrajectory:     &currentTrajectory,
		IncomingGoalRequest:   make(chan float64, 10), // Buffered to prevent blocking
		State:                 Idle,
		PendingRequests:       make(map[int64]*RequestParameters),
	}
}

// run_controller starts the controller's main loop
func (c *Controller) run_controller() {

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	// for loop using ticker
	for {
		select {
		case goal := <-c.IncomingGoalRequest:
			if c.State == Idle {
				c.handleGoalRequest(goal, Moving)
			} else {
				fmt.Println(c.Cart.Name, ": Ignoring goal request while not idle.")
			}

		case request := <-c.IncomingRightRequest:
			c.handleIncomingRequest(request, Right)
		case request := <-c.IncomingLeftRequest:
			c.handleIncomingRequest(request, Left)

		case <-ticker.C:
			// run PID controllers
			c.runPIDControllers()

			// state machine
			switch c.State {
			case Busy:
				if time.Now().After(c.BusyUntil) {
					fmt.Println(c.Cart.Name, ": Busy period ended, returning to idle state.")
					c.State = Idle
				}
			case Moving:
				// Check if the cart has reached the goal
				if c.CurrentTrajectory.IsFinished() {
					fmt.Println(c.Cart.Name, ": Goal reached!")
					c.State = Busy
					c.BusyUntil = time.Now().Add(1000 * time.Millisecond) // Simulate busy state for 1s
				}
			case Avoiding:
				// Check if the cart has reached the goal
				if c.CurrentTrajectory.IsFinished() {
					fmt.Println(c.Cart.Name, ": Goal reached!")
					c.State = Idle
				}
			case Requesting:
				// check if there are any responses from neighbors
				select {
				case response := <-c.IncomingRightResponse:
					c.handleResponse(response, Right)
				case response := <-c.IncomingLeftResponse:
					c.handleResponse(response, Left)
				default:
					// No response, continue without blocking
				}
				// check if there are any requests to retry
				c.retryPendingRequests()
			case Idle:
				// check if there are any requests to retry
				c.retryPendingRequests()
			}
		}
	}
}

func (c *Controller) retryPendingRequests() {
	for requestId, pendingRequest := range c.PendingRequests {
		if time.Now().After(pendingRequest.RetryTime) {
			fmt.Println(c.Cart.Name, ": Retrying request with ID:", pendingRequest.Request.RequestId)
			// Remove the old entry since queueBorderMoveRequest will add a new one
			delete(c.PendingRequests, requestId)
			// Retry the request
			c.queueBorderMoveRequest(
				pendingRequest.Goal,
				pendingRequest.AcceptState,
				&requestId, // Pass the old request ID for retry
				pendingRequest.OriginalRequest,
				pendingRequest.OriginalUpdateTrajectory,
				pendingRequest.OriginalOutgoingResponse,
			)
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
	c.handleGoalRequestWithOriginal(goal, acceptState, nil, nil, nil)
}

func (c *Controller) handleGoalRequestWithOriginal(goal float64, acceptState State, originalRequest *Request, originalUpdateTrajectory func(*Trajectory), originalOutgoingResponse chan Response) {
	fmt.Println(c.Cart.Name, ": received goal request:", goal)

	if c.LeftBorderTrajectory.end+c.safetyMargin < goal && goal < c.RightBorderTrajectory.end-c.safetyMargin {
		c.acceptGoal(goal, acceptState)
	} else {
		c.queueBorderMoveRequest(goal, acceptState, nil, originalRequest, originalUpdateTrajectory, originalOutgoingResponse)
	}
}

func (c *Controller) acceptGoal(goal float64, acceptState State) {
	fmt.Println(c.Cart.Name, ": Goal accepted:", goal)
	c.State = acceptState
	// Handle incoming goal request
	trajectory := c.MovementPlanner.CalculateTrajectory(c.Cart.Position, goal)
	c.CurrentTrajectory = &trajectory
}

func (c *Controller) rejectGoal(goal float64) {
	fmt.Println(c.Cart.Name, ": Goal permanently rejected:", goal)
	c.State = Idle
}

func (c *Controller) postponeGoal(goal float64) {
	fmt.Println(c.Cart.Name, ": Goal postponed:", goal)
}

func (c *Controller) queueBorderMoveRequest(goal float64, acceptState State, oldRequestId *int64, originalRequest *Request, originalUpdateTrajectory func(*Trajectory), originalOutgoingResponse chan Response) {
	fmt.Println(c.Cart.Name, ": Goal out of bounds, queuing border move request:", goal)
	c.State = Requesting

	// Helper function to handle border move requests
	trySendRequest := func(outgoing chan Request, start, end float64) {
		if outgoing == nil {
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
			} else {
				requestId = time.Now().UnixNano() // Unique request ID based on current time
			}
			request := Request{RequestId: requestId, ProposedBorderStart: start, ProposedBorderEnd: end}
			requestParameters := RequestParameters{
				Goal:                     goal,
				Request:                  request,
				RetryTime:                time.Now().Add(1000 * time.Millisecond),
				AcceptState:              acceptState, // State to transition to if the request is accepted
				OriginalRequest:          originalRequest,
				OriginalUpdateTrajectory: originalUpdateTrajectory,
				OriginalOutgoingResponse: originalOutgoingResponse,
			}
			c.PendingRequests[requestId] = &requestParameters
			// Send the request to the neighbor controller
			outgoing <- request
		}
	}

	if c.LeftBorderTrajectory.end+c.safetyMargin >= goal {
		trySendRequest(
			c.OutgoingLeftRequest,
			c.LeftBorderTrajectory.end,
			goal-1.01*c.safetyMargin,
		)
	} else if c.RightBorderTrajectory.end-c.safetyMargin <= goal {
		trySendRequest(
			c.OutgoingRightRequest,
			c.RightBorderTrajectory.end,
			goal+1.01*c.safetyMargin,
		)
	} else {
		// unreachable
		fmt.Println(c.Cart.Name, ": Error: Goal inside bounds but still got to processing border move request.")
	}
}

func (c *Controller) handleResponse(response Response, side Side) {
	requestParams, exists := c.PendingRequests[response.RequestId]
	if !exists {
		fmt.Println(c.Cart.Name, ": Received response for unknown or old request, ignoring.")
		return
	}

	// Remove the request from pending requests after handling
	delete(c.PendingRequests, response.RequestId)

	switch response.Type {
	case ACCEPT:
		c.handleAcceptResponse(*requestParams, side)
	case REJECT:
		c.handleRejectResponse(*requestParams, side)
	case WAIT:
		c.handleWaitResponse(*requestParams, side)
	default:
		fmt.Println(c.Cart.Name, ": Unknown response received.")
	}
}

func (c *Controller) handleAcceptResponse(requestParams RequestParameters, side Side) {
	fmt.Println(c.Cart.Name, ": Border move request accepted.")
	// Update the border trajectory based on the accepted request
	if side == Left {
		trajectory := c.MovementPlanner.CalculateTrajectory(
			requestParams.Request.ProposedBorderStart,
			requestParams.Request.ProposedBorderEnd,
		)
		c.LeftBorderTrajectory = &trajectory
	} else {
		trajectory := c.MovementPlanner.CalculateTrajectory(
			requestParams.Request.ProposedBorderStart,
			requestParams.Request.ProposedBorderEnd,
		)
		c.RightBorderTrajectory = &trajectory
	}

	// Accept the goal and start moving towards it
	c.acceptGoal(requestParams.Goal, requestParams.AcceptState)

	// If this was triggered by an original request, accept that request too
	if requestParams.OriginalRequest != nil && requestParams.OriginalUpdateTrajectory != nil && requestParams.OriginalOutgoingResponse != nil {
		c.acceptRequest(requestParams.OriginalUpdateTrajectory, requestParams.OriginalOutgoingResponse, *requestParams.OriginalRequest)
	}
}

func (c *Controller) handleRejectResponse(requestParams RequestParameters, side Side) {
	fmt.Println(c.Cart.Name, ": Border move request rejected.")
	// Reject the goal and stop moving
	c.rejectGoal(requestParams.Goal)

	// If this was triggered by an original request, reject that request too
	if requestParams.OriginalRequest != nil && requestParams.OriginalOutgoingResponse != nil {
		c.rejectRequest(requestParams.OriginalOutgoingResponse, *requestParams.OriginalRequest)
	}
}

func (c *Controller) handleWaitResponse(requestParams RequestParameters, side Side) {
	fmt.Println(c.Cart.Name, ": Border move request waiting for response.")
	requestParams.RetryTime = time.Now().Add(1000 * time.Millisecond) // Retry after 1000ms
	c.PendingRequests[requestParams.Request.RequestId] = &requestParams
	c.postponeGoal(requestParams.Goal)

	// If this was triggered by an original request, postpone that request too
	if requestParams.OriginalRequest != nil && requestParams.OriginalUpdateTrajectory != nil && requestParams.OriginalOutgoingResponse != nil {
		c.postponeRequest(requestParams.OriginalOutgoingResponse, *requestParams.OriginalRequest)
	}
}

func (c *Controller) handleIncomingRequest(request Request, side Side) {
	fmt.Println(c.Cart.Name, ": Received border move request from neighbor:", request)

	if side == Left {
		acceptImmediately := request.ProposedBorderEnd < c.CurrentTrajectory.end-c.safetyMargin
		c.handleBorderMove(
			acceptImmediately,
			func(t *Trajectory) { c.LeftBorderTrajectory = t },
			c.OutgoingLeftResponse,
			request,
			side,
		)
	} else {
		acceptImmediately := request.ProposedBorderEnd > c.CurrentTrajectory.end+c.safetyMargin
		c.handleBorderMove(
			acceptImmediately,
			func(t *Trajectory) { c.RightBorderTrajectory = t },
			c.OutgoingRightResponse,
			request,
			side,
		)
	}
}

func (c *Controller) handleBorderMove(
	acceptImmediately bool,
	updateTrajectory func(*Trajectory),
	outgoingResponse chan Response,
	request Request,
	side Side,
) {
	fmt.Println(c.Cart.Name, ": Handling border move request:", request)
	if acceptImmediately {
		c.acceptRequest(updateTrajectory, outgoingResponse, request)
	} else {
		c.tryToGiveWay(updateTrajectory, outgoingResponse, request, side)
	}
}

func (c *Controller) acceptRequest(updateTrajectory func(*Trajectory), outgoingResponse chan Response, request Request) {
	fmt.Println(c.Cart.Name, ": Accepting border move request:", request)
	trajectory := c.MovementPlanner.CalculateTrajectory(
		request.ProposedBorderStart,
		request.ProposedBorderEnd,
	)
	updateTrajectory(&trajectory)
	outgoingResponse <- Response{
		RequestId: request.RequestId,
		Type:      ACCEPT,
	}
}

func (c *Controller) rejectRequest(outgoingResponse chan Response, request Request) {
	fmt.Println(c.Cart.Name, ": Rejecting border move request:", request)
	outgoingResponse <- Response{
		RequestId: request.RequestId,
		Type:      REJECT,
	}
}

func (c *Controller) postponeRequest(outgoingResponse chan Response, request Request) {
	fmt.Println(c.Cart.Name, ": Postponing border move request:", request)
	outgoingResponse <- Response{
		RequestId: request.RequestId,
		Type:      WAIT,
	}
}

func (c *Controller) tryToGiveWay(updateTrajectory func(*Trajectory), outgoingResponse chan Response, request Request, side Side) {
	fmt.Println(c.Cart.Name, ": Trying to give way to neighbor's request:", request)

	avoidanceGoal := 0.0
	if side == Left {
		avoidanceGoal = request.ProposedBorderEnd + 1.01*c.safetyMargin
	} else {
		avoidanceGoal = request.ProposedBorderEnd - 1.01*c.safetyMargin
	}

	if c.State == Idle || c.State == Requesting {
		fmt.Println(c.Cart.Name, ": Giving way to neighbor's request:", request)

		// Check if the avoidance goal is within current borders
		canAvoidImmediately := c.LeftBorderTrajectory.end+c.safetyMargin < avoidanceGoal && avoidanceGoal < c.RightBorderTrajectory.end-c.safetyMargin

		if canAvoidImmediately {
			// Avoidance maneuver is immediately successful, accept the original request
			c.handleGoalRequest(avoidanceGoal, Avoiding)
			c.acceptRequest(updateTrajectory, outgoingResponse, request)
		} else {
			// Avoidance maneuver requires border expansion, forward the response from that process
			c.handleGoalRequestWithOriginal(avoidanceGoal, Avoiding, &request, updateTrajectory, outgoingResponse)
		}
	} else {
		fmt.Println(c.Cart.Name, ": Cannot give way while not idle, postponing request.")
		c.postponeRequest(outgoingResponse, request)
	}
}
