package main

import (
	"fmt"
	"math/rand/v2"
	"time"
)

type RequestResult int

const (
	RequestSuccess RequestResult = iota
	RequestWait
	RequestReject
	RequestTimeout
)

type ControllerState int

const (
	Idle ControllerState = iota
	Busy
	Requesting
	Moving
	Avoiding
)

func (s ControllerState) String() string {
	switch s {
	case Idle:
		return "Idle"
	case Busy:
		return "Busy"
	case Requesting:
		return "Requesting"
	case Moving:
		return "Moving"
	case Avoiding:
		return "Avoiding"
	default:
		return "Unknown"
	}
}

type Controller struct {
	// provide direct access to the cart's physical properties
	// "intrinsic sensing and perfect actuation", good enough for our purposes
	Cart *Cart

	// PID controllers for velocity and position
	VelocityPID     *PID
	PositionPID     *PID
	AccelerationPID *PID

	// MovementPlanner controller
	MovementPlanner *MovementPlanner
	MPCTrajectory   Trajectory

	// The current state
	State        ControllerState
	Goal         float64
	HasGoal      bool
	GoalPriority time.Time
	BusyUntil    time.Time // Used to track busy state duration

	// The cart's old goal while avoiding
	OldGoal    float64
	HasOldGoal bool

	NextGoal         float64
	HasNextGoal      bool
	NextGoalPriority time.Time // Priority of the next goal

	// Current bounds
	LeftBound    Trajectory
	RightBound   Trajectory
	SafetyMargin float64

	// Goal retry mechanism for WAIT responses
	PendingGoal      float64
	PendingRetryTime time.Time
	HasPendingGoal   bool

	// Channel for accepting and responding to goal requests
	IncomingGoalRequest <-chan float64

	// Channels for communication with the neighboring carts
	IncomingLeftRequest   <-chan Request
	IncomingRightRequest  <-chan Request
	OutgoingLeftRequest   chan<- Request
	OutgoingRightRequest  chan<- Request
	IncomingLeftResponse  <-chan Response
	IncomingRightResponse <-chan Response
	OutgoingLeftResponse  chan<- Response
	OutgoingRightResponse chan<- Response
}

// NewController creates a new controller with the given PID controllers
func NewController(cart *Cart, leftBound, rightBound float64) *Controller {
	movementPlanner := NewMovementPlanner(200, 100, 300)

	return &Controller{
		Cart:            cart,
		Goal:            cart.Position,
		VelocityPID:     NewPID(150, 0, 0, 0.01), // Reduced Kp, added Ki, reduced Kd
		PositionPID:     NewPID(100, 0, 0, 0.01), // Reduced Kp, added Ki, increased Kd
		MovementPlanner: NewMovementPlanner(200, 100, 300),
		State:           Idle,
		LeftBound:       movementPlanner.CalculateTrajectory(leftBound, leftBound),
		RightBound:      movementPlanner.CalculateTrajectory(rightBound, rightBound),
		SafetyMargin:    40,
	}
}

func (controller *Controller) acceptGoal(goal float64, acceptState ControllerState) {
	// accept the goal
	fmt.Println(controller.Cart.Name, ": Goal accepted: ", goal)
	controller.Goal = goal
	controller.HasGoal = true
	fmt.Println(controller.Cart.Name, ": State changed from ", controller.State.String(), " to ", acceptState.String())
	controller.State = acceptState

	// execute the MPC trajectory
	trajectory := controller.MovementPlanner.CalculateTrajectory(controller.Cart.Position, controller.Goal)
	controller.MPCTrajectory = trajectory
}

func (controller *Controller) boundedPositionSetpoint() float64 {
	unboundedPositionSetpoint := controller.MPCTrajectory.GetCurrentPosition()
	// leftBound := controller.LeftBound.GetCurrentPosition()
	// rightBound := controller.RightBound.GetCurrentPosition()
	// return clamp(unboundedPositionSetpoint, leftBound+controller.SafetyMargin, rightBound-controller.SafetyMargin)
	return unboundedPositionSetpoint
}

func (controller *Controller) requestBoundMove(proposedBorder Trajectory, outgoingRequest chan<- Request, incomingResponse <-chan Response) RequestResult {
	if controller.State == Avoiding {
		fmt.Println(controller.Cart.Name, ": Already avoiding, cannot request bound move")
		return RequestReject
	}

	fmt.Println(controller.Cart.Name, ": State changed from ", controller.State.String(), " to ", Requesting.String())
	controller.State = Requesting

	fmt.Println(controller.Cart.Name, ": Requesting to move border.")

	if outgoingRequest == nil {
		fmt.Println(controller.Cart.Name, ": Hard bound, rejecting request locally")
		return RequestReject
	}

	request := Request{
		proposed_border: proposedBorder,
		priority:        controller.GoalPriority,
	}

	select {
	case outgoingRequest <- request:
		response := <-incomingResponse
		switch response {
		case WAIT:
			fmt.Println(controller.Cart.Name, ": Request denied - told to wait")
			return RequestWait
		case ACCEPT:
			return RequestSuccess
		case REJECT:
			return RequestReject
		}
	case <-time.After(1000 * time.Millisecond):
		fmt.Println(controller.Cart.Name, ": Request timed out")
		return RequestTimeout
	}
	return RequestReject // unreachable but needed to satisfy the compiler
}

func (controller *Controller) rejectGoal(rejectState ControllerState) {
	// if the left bound is rejected, reject the goal
	fmt.Println(controller.Cart.Name, ": Goal rejected: ", controller.Goal)
	controller.State = rejectState
}

func (controller *Controller) processGoal(goal float64, acceptState ControllerState, rejectState ControllerState) bool {

	fmt.Println(controller.Cart.Name, ": Processing goal: ", goal)

	leftBound := controller.LeftBound.end
	rightBound := controller.RightBound.end

	// check if the goal is within bounds
	if goal < leftBound+controller.SafetyMargin {
		// if we have a cart to the left, ask to move the left bound
		proposedLeftBorder := goal - controller.SafetyMargin
		proposedTrajectoryMove := controller.MovementPlanner.CalculateTrajectory(leftBound, proposedLeftBorder)
		result := controller.requestBoundMove(proposedTrajectoryMove, controller.OutgoingLeftRequest, controller.IncomingLeftResponse)
		switch result {
		case RequestSuccess:
			proposedTrajectoryMove.t0 = time.Now() // reset the start time of the trajectory
			controller.LeftBound = proposedTrajectoryMove
			controller.acceptGoal(goal, acceptState)
			return true
		case RequestWait, RequestTimeout:
			// Temporary failure - set up retry in 3 - 5 seconds
			retryDelay := time.Duration(rand.IntN(2000)+3000) * time.Millisecond
			controller.PendingGoal = goal
			controller.PendingRetryTime = time.Now().Add(retryDelay)
			controller.HasPendingGoal = true
			fmt.Println(controller.Cart.Name, ": Goal temporarily rejected (WAIT), will retry in ", retryDelay)
			controller.rejectGoal(rejectState)
			return false
		default: // RequestReject or RequestTimeout
			// Permanent failure - give up
			fmt.Println(controller.Cart.Name, ": Goal permanently rejected")
			controller.rejectGoal(rejectState)
			return false
		}

	} else if goal > rightBound-controller.SafetyMargin {
		// if we have a cart to the right, ask to move the right bound
		proposedRightBorder := goal + controller.SafetyMargin
		proposedTrajectoryMove := controller.MovementPlanner.CalculateTrajectory(rightBound, proposedRightBorder)
		result := controller.requestBoundMove(proposedTrajectoryMove, controller.OutgoingRightRequest, controller.IncomingRightResponse)
		switch result {
		case RequestSuccess:
			proposedTrajectoryMove.t0 = time.Now() // reset the start time of the trajectory
			controller.RightBound = proposedTrajectoryMove
			controller.acceptGoal(goal, acceptState)
			return true
		case RequestWait, RequestTimeout:
			// Temporary failure - set up retry in 3 - 5 seconds
			retryDelay := time.Duration(rand.IntN(2000)+3000) * time.Millisecond
			controller.PendingGoal = goal
			controller.PendingRetryTime = time.Now().Add(retryDelay)
			controller.HasPendingGoal = true
			fmt.Println(controller.Cart.Name, ": Goal temporarily rejected (WAIT), will retry in ", retryDelay)
			controller.rejectGoal(rejectState)
			return false
		default: // RequestReject or RequestTimeout
			// Permanent failure - give up
			fmt.Println(controller.Cart.Name, ": Goal permanently rejected")
			controller.rejectGoal(rejectState)
			return false
		}
	} else {
		// accept the goal
		controller.acceptGoal(goal, acceptState)
		return true
	}
}

func (controller *Controller) avoidCollision(newGoal float64, outgoingResponse chan<- Response) bool {
	if controller.processGoal(newGoal, Avoiding, controller.State) {
		fmt.Println(controller.Cart.Name, ": Emergency stop complete, avoiding collision with new goal: ", newGoal)
		outgoingResponse <- ACCEPT
		return true
	} else {
		fmt.Println(controller.Cart.Name, ": Emergency stop complete but cannot avoid collision, rejecting request: ", newGoal)
		outgoingResponse <- REJECT
		return false
	}
}

func (controller *Controller) maybeQueueAvoidance(proposedBorder Trajectory, priority time.Time, isLeft bool) {
	newGoal := 0.0
	if isLeft {
		newGoal = proposedBorder.end + 1.1*controller.SafetyMargin // Set next goal to avoid collision
	} else {
		newGoal = proposedBorder.end - 1.1*controller.SafetyMargin // Set next goal to avoid collision
	}

	// if the controller has a next goal, update it if the new one has a lower priority
	if !controller.HasNextGoal || priority.Before(controller.NextGoalPriority) {
		controller.NextGoal = newGoal
		controller.HasNextGoal = true
		controller.OutgoingLeftResponse <- Response(WAIT)
	}
}

func (controller *Controller) run_controller() {

	fmt.Println(controller.Cart.Name, ": Starting controller")

	// first accept the goal to initial position (to initialize the MPC)
	controller.acceptGoal(controller.Cart.Position, Idle)

	// Run the PIDs in a separate goroutine
	go func() {
		ticker := time.NewTicker(10 * time.Millisecond) // 100Hz
		defer ticker.Stop()
		for range ticker.C {
			// PID control updates
			controller.PositionPID.SetSetpoint(controller.boundedPositionSetpoint())
			control_velocity := controller.PositionPID.Update(controller.Cart.Position)
			controller.VelocityPID.SetSetpoint(control_velocity)
			control_force := controller.VelocityPID.Update(controller.Cart.Velocity)
			controller.Cart.applyForce(control_force)
		}
	}()

	// Single ticker for all control loops (100Hz)
	ticker := time.NewTicker(time.Second / 100)
	defer ticker.Stop()

	for {
		select {
		// Handle incoming left requests
		case request := <-controller.IncomingLeftRequest:
			proposedLeftBorder := request.proposed_border
			proposedLeftBorder.t0 = time.Now() // reset the start time of the trajectory
			fmt.Println(controller.Cart.Name, ": Received request to move left bound to ", proposedLeftBorder.end)
			if controller.Goal > proposedLeftBorder.end+controller.SafetyMargin {
				fmt.Println(controller.Cart.Name, ": Accepting request to move left bound.")
				controller.OutgoingLeftResponse <- Response(ACCEPT)
				controller.LeftBound = proposedLeftBorder
			} else if controller.State == Idle {
				// we either have a goal and are waiting to retry the request,
				// or have no goals.

				// If we do have a goal, we only accept the request if the priority is higher than our current goal
				if !controller.HasGoal || request.priority.Before(controller.GoalPriority) {
					if controller.avoidCollision(proposedLeftBorder.end+1.1*controller.SafetyMargin, controller.OutgoingLeftResponse) {
						controller.LeftBound = proposedLeftBorder
					}
				}
			} else if controller.State == Moving && request.priority.Before(controller.GoalPriority) {
				// fmt.Println(controller.Cart.Name, ": Yielding right of way to left bound request.")
				// // if the other cart's goal is older than our goal, we yield right of way to it
				// // otherwise we tell it to wait until we complete our goal
				// controller.emergencyStop(controller.OutgoingLeftResponse)
				fmt.Println(controller.Cart.Name, ": Cannot accept request to move left bound because we're moving, rejecting request.")
				controller.NextGoal = proposedLeftBorder.end + 1.1*controller.SafetyMargin // Set next goal to avoid collision
				controller.HasNextGoal = true
				controller.OutgoingLeftResponse <- Response(WAIT)
			} else {
				fmt.Println(controller.Cart.Name, ": Cannot accept request to move left bound, rejecting request.")
				controller.OutgoingLeftResponse <- Response(WAIT)
			}

		// Handle incoming right requests
		case request := <-controller.IncomingRightRequest:
			proposedRightBorder := request.proposed_border
			proposedRightBorder.t0 = time.Now() // reset the start time of the trajectory
			fmt.Println(controller.Cart.Name, ": Received request to move right bound to  ", proposedRightBorder.end)
			if controller.Goal < proposedRightBorder.end-controller.SafetyMargin {
				fmt.Println(controller.Cart.Name, ": Accepting request to move right bound.")
				controller.RightBound = proposedRightBorder // TODO: unsafe
				controller.OutgoingRightResponse <- Response(ACCEPT)
			} else if controller.State == Idle {
				// we have come to an emergency stop, so we can accept the request
				if controller.avoidCollision(proposedRightBorder.end-1.1*controller.SafetyMargin, controller.OutgoingRightResponse) {
					controller.RightBound = proposedRightBorder
				}
			} else if controller.State == Moving && request.priority.Before(controller.GoalPriority) {
				// fmt.Println(controller.Cart.Name, ": Yielding right of way to right bound request.")
				// // if the other cart's goal is older than our goal, we yield right of way to it
				// // otherwise we tell it to wait until we complete our goal
				// controller.emergencyStop(controller.OutgoingRightResponse)
				fmt.Println(controller.Cart.Name, ": Cannot accept request to move right bound because we're moving, rejecting request.")
				controller.NextGoal = proposedRightBorder.end - 1.1*controller.SafetyMargin // Set next goal to avoid collision
				controller.HasNextGoal = true
				controller.OutgoingRightResponse <- Response(WAIT)
			} else {
				fmt.Println(controller.Cart.Name, ": Cannot accept request to move right bound, rejecting request.")
				controller.OutgoingRightResponse <- Response(WAIT)
			}

		// Main control loop - runs at 100Hz
		case <-ticker.C:

			// State machine updates
			switch controller.State {
			case Idle:
				// Check for pending goal retry
				if controller.HasPendingGoal && time.Now().After(controller.PendingRetryTime) {
					fmt.Println(controller.Cart.Name, ": Retrying pending goal: ", controller.PendingGoal)
					controller.HasPendingGoal = false
					controller.processGoal(controller.PendingGoal, Moving, Idle)
				} else if !controller.HasPendingGoal {
					if controller.HasNextGoal {
						fmt.Println(controller.Cart.Name, ": Processing next goal: ", controller.NextGoal)
						controller.processGoal(controller.NextGoal, Avoiding, Idle)
						controller.HasNextGoal = false // Clear the next goal after processing
					} else {

						// check if we have a new goal on the incoming goal channel
						select {
						case goal := <-controller.IncomingGoalRequest:
							controller.GoalPriority = time.Now()
							// pass the goal to the processGoal function
							controller.processGoal(goal, Moving, Idle)
						default:
							// no new goal, do nothing
						}
					}
				}
			case Busy:
				if time.Now().After(controller.BusyUntil) {
					controller.State = Idle
					fmt.Println(controller.Cart.Name, ": State changed from ", Busy.String(), " to ", Idle.String())
				}
			case Requesting:
				// The request is being handled in requestBoundMove function
				// This state will be exited when the function completes
			case Moving:
				// check if we have reached the goal
				if controller.MPCTrajectory.IsFinished() {
					// goal reached, go to busy for a random time
					controller.State = Busy
					controller.HasGoal = false                                                                    // Clear the goal after reaching it
					controller.BusyUntil = time.Now().Add(time.Duration(rand.IntN(2000)+1000) * time.Millisecond) // busy for 1 to 3 second
					fmt.Println(controller.Cart.Name, ": State changed from ", Moving.String(), " to ", Busy.String())
				}
			case Avoiding:
				if controller.MPCTrajectory.IsFinished() {
					if controller.HasOldGoal {
						controller.processGoal(controller.OldGoal, Moving, Idle)
						controller.HasOldGoal = false // Clear the old goal after processing
					} else {
						controller.State = Idle
					}
				}
			}
		}
	}
}
