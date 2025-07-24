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
)

func (s State) String() string {
	switch s {
	case Idle:
		return "Idle"
	case Moving:
		return "Moving"
	case Requesting:
		return "Requesting"
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
	Goal      float64
	Request   Request
	RetryTime time.Time // Time when the request is to be retried
}

type Controller struct {
	Cart                  *Cart
	LeftBorderTrajectory  *Trajectory
	RightBorderTrajectory *Trajectory
	CurrentTrajectory     *Trajectory
	State

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

	// Queued requests mapped by RequestId
	QueuedRequests map[int64]RequestParameters
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
		safetyMargin:          50, // Example safety margin
		LeftBorderTrajectory:  &leftBorderTrajectory,
		RightBorderTrajectory: &rightBorderTrajectory,
		CurrentTrajectory:     &currentTrajectory,
		IncomingGoalRequest:   make(chan float64), // Buffered to prevent blocking
		State:                 Idle,
		QueuedRequests:        make(map[int64]RequestParameters),
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
			c.handleGoalRequest(goal)

		case request := <-c.IncomingRightRequest:
			c.handleIncomingRequest(request, Right)
		case request := <-c.IncomingLeftRequest:
			c.handleIncomingRequest(request, Left)

		case <-ticker.C:
			// run PID controllers
			c.runPIDControllers()

			// state machine
			switch c.State {
			case Moving:
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
				for requestId, requestParams := range c.QueuedRequests {
					if time.Now().After(requestParams.RetryTime) {
						fmt.Println(c.Cart.Name, ": Retrying request with ID:", requestId)
						// Retry the request
						c.queueBorderMoveRequest(requestParams.Goal)
						delete(c.QueuedRequests, requestId) // Remove the request after retrying
					}
				}
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

func (c *Controller) handleGoalRequest(goal float64) {
	fmt.Println(c.Cart.Name, ": received goal request:", goal)

	if c.State != Idle {
		fmt.Println(c.Cart.Name, ": Ignoring goal request while not idle.")
		return
	}

	if c.LeftBorderTrajectory.end+c.safetyMargin < goal && goal < c.RightBorderTrajectory.end-c.safetyMargin {
		c.acceptGoal(goal)
	} else {
		c.queueBorderMoveRequest(goal)
	}
}

func (c *Controller) acceptGoal(goal float64) {
	fmt.Println(c.Cart.Name, ": Goal accepted:", goal)
	c.State = Moving
	// Handle incoming goal request
	trajectory := c.MovementPlanner.CalculateTrajectory(c.Cart.Position, goal)
	c.CurrentTrajectory = &trajectory
}

func (c *Controller) rejectGoal(goal float64) {
	fmt.Println(c.Cart.Name, ": Goal permanently rejected:", goal)
	c.State = Idle
}

func (c *Controller) queueBorderMoveRequest(goal float64) {
	fmt.Println(c.Cart.Name, ": Goal out of bounds, queuing border move request:", goal)
	c.State = Requesting

	// Helper function to handle border move requests
	trySendRequest := func(outgoing chan Request, start, end float64) {
		if outgoing == nil {
			// No neighbor, reject request
			c.rejectGoal(goal)
		} else {
			requestId := time.Now().UnixNano() // Unique request ID based on current time
			request := Request{RequestId: requestId, ProposedBorderStart: start, ProposedBorderEnd: end}
			requestParameters := RequestParameters{
				Goal:      goal,
				Request:   request,
				RetryTime: time.Now().AddDate(1000, 0, 0), // basically don't retry
			}
			c.QueuedRequests[requestId] = requestParameters
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
	if _, exists := c.QueuedRequests[response.RequestId]; !exists {
		fmt.Println(c.Cart.Name, ": Error: Received response for unknown request ID:", response.RequestId)
		return
	}
	requestParams := c.QueuedRequests[response.RequestId]
	delete(c.QueuedRequests, response.RequestId)

	switch response.Type {
	case ACCEPT:
		c.handleAcceptResponse(requestParams, side)
	case REJECT:
		c.handleRejectResponse(requestParams, side)
	case WAIT:
		c.handleWaitResponse(requestParams, side)
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
	c.acceptGoal(requestParams.Goal)
}

func (c *Controller) handleRejectResponse(requestParams RequestParameters, side Side) {
	fmt.Println(c.Cart.Name, ": Border move request rejected.")
	// Reject the goal and stop moving
	c.rejectGoal(requestParams.Goal)
}

func (c *Controller) handleWaitResponse(requestParams RequestParameters, side Side) {
	fmt.Println(c.Cart.Name, ": Border move request waiting for response.")
	requestParams.RetryTime = time.Now().Add(1000 * time.Millisecond) // Retry after 1000ms
	c.QueuedRequests[requestParams.Request.RequestId] = requestParams
}

func (c *Controller) handleIncomingRequest(request Request, side Side) {
	fmt.Println(c.Cart.Name, ": Received border move request from neighbor:", request)

	if side == Left {
		if request.ProposedBorderEnd < c.CurrentTrajectory.end-c.safetyMargin {
			fmt.Println(c.Cart.Name, ": Proposed border end is", request.ProposedBorderEnd, "while we would accept anything less than", c.CurrentTrajectory.end-c.safetyMargin)
			c.acceptRequest(
				func(t *Trajectory) { c.LeftBorderTrajectory = t },
				c.OutgoingLeftResponse,
				request,
			)
		} else {
			c.postponeRequest(c.OutgoingLeftResponse, request)
		}
	} else {
		if request.ProposedBorderEnd > c.CurrentTrajectory.end+c.safetyMargin {
			fmt.Println(c.Cart.Name, ": Proposed border end is", request.ProposedBorderEnd, "while we would accept anything more than", c.CurrentTrajectory.end+c.safetyMargin)
			c.acceptRequest(
				func(t *Trajectory) { c.RightBorderTrajectory = t },
				c.OutgoingRightResponse,
				request,
			)
		} else {
			c.postponeRequest(c.OutgoingRightResponse, request)
		}
	}
}

func (c *Controller) acceptRequest(updateTrajectory func(*Trajectory), outgoingResponse chan Response, request Request) {
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
	outgoingResponse <- Response{
		RequestId: request.RequestId,
		Type:      REJECT,
	}
}

func (c *Controller) postponeRequest(outgoingResponse chan Response, request Request) {
	outgoingResponse <- Response{
		RequestId: request.RequestId,
		Type:      WAIT,
	}
}
