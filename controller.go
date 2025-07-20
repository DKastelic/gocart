package main

import (
	"fmt"
	"math"
	"math/rand/v2"
	"time"
)

type ControllerState int

const (
	Idle ControllerState = iota
	Processing
	Requesting
	Moving
	Avoiding
)

func (s ControllerState) String() string {
	switch s {
	case Idle:
		return "Idle"
	case Processing:
		return "Processing"
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
	GoalReceived time.Time

	// The cart's old goal while avoiding
	OldState ControllerState
	OldGoal  float64

	// Current bounds
	LeftBound    float64
	RightBound   float64
	SafetyMargin float64

	// Channel for accepting and responding to goal requests
	IncomingGoalRequest  <-chan float64
	OutgoingGoalResponse chan<- Response

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
	return &Controller{
		Cart:            cart,
		Goal:            cart.Position,
		VelocityPID:     NewPID(20, 0, 0.2, 0.01),
		PositionPID:     NewPID(3, 0, 0.03, 0.01),
		MovementPlanner: NewMovementPlanner(200, 100, 300),
		State:           Idle,
		LeftBound:       leftBound,
		RightBound:      rightBound,
		SafetyMargin:    30,
	}
}

func (controller *Controller) checkForNewGoal() {
	select {
	case goal := <-controller.IncomingGoalRequest:
		controller.processGoal(goal)
	default:
		// if the channel is empty, do nothing
	}
}

func (controller *Controller) acceptGoal(goal float64) {
	// accept the goal
	fmt.Println(controller.Cart.Name, ": Goal accepted: ", goal)
	controller.Goal = goal
	controller.State = Moving

	// execute the MPC trajectory
	trajectory := controller.MovementPlanner.CalculateTrajectory(controller.Cart.Position, controller.Goal)
	controller.MPCTrajectory = trajectory
}

func (controller *Controller) boundedPositionSetpoint() float64 {
	unboundedPositionSetpoint := controller.MovementPlanner.GetCurrentTrajectoryPosition(controller.MPCTrajectory)
	return unboundedPositionSetpoint // TODO: clamp to bounds once bounds are moving too or something
	// return math.Min(math.Max(unboundedPositionSetpoint, controller.LeftBound+controller.SafetyMargin), controller.RightBound-controller.SafetyMargin)
}

func (controller *Controller) requestBoundMove(proposedBorder float64, outgoingRequest chan<- Request, incomingResponse <-chan Response) bool {
	if controller.State == Avoiding {
		fmt.Println(controller.Cart.Name, ": Already avoiding, cannot request bound move")
		return false
	}

	controller.State = Requesting

	fmt.Println(controller.Cart.Name, ": Requesting to move border to: ", proposedBorder)

	if outgoingRequest == nil {
		fmt.Println(controller.Cart.Name, ": Hard bound, rejecting request locally")
		return false
	}

	request := Request{
		proposed_border: proposedBorder,
		priority:        controller.GoalReceived,
	}

	select {
	case outgoingRequest <- request:
		response := <-incomingResponse
		switch response {
		case WAIT:
			d := rand.IntN(700) + 300 // wait between 0.3 and 1 seconds
			fmt.Println(controller.Cart.Name, ": Waiting for response, will retry in ", d, "ms")
			fmt.Println(controller.Cart.Name, ": Waiting")
			time.Sleep(time.Duration(d) * time.Millisecond)
			return controller.requestBoundMove(proposedBorder, outgoingRequest, incomingResponse)
		case ACCEPT:
			return true
		case REJECT:
			return false
		}
	case <-time.After(1000 * time.Millisecond):
		fmt.Println(controller.Cart.Name, ": Request timed out")
		return false
	}
	return false // unreachable but needed to satisfy the compiler
}

func (controller *Controller) processGoal(goal float64) bool {

	fmt.Println(controller.Cart.Name, ": Processing goal: ", goal)
	controller.State = Processing

	controller.GoalReceived = time.Now()

	// check if the goal is within bounds
	if goal < controller.LeftBound+controller.SafetyMargin {
		// if we have a cart to the left, ask to move the left bound
		proposedLeftBorder := goal - controller.SafetyMargin
		success := controller.requestBoundMove(proposedLeftBorder, controller.OutgoingLeftRequest, controller.IncomingLeftResponse)
		if success {
			controller.LeftBound = proposedLeftBorder
			controller.acceptGoal(goal)
		} else {
			// if the left bound is rejected, reject the goal
			fmt.Println(controller.Cart.Name, ": Left bound rejected, goal rejected: ", goal)
			controller.State = Idle
		}
		return success

	} else if goal > controller.RightBound-controller.SafetyMargin {
		// if we have a cart to the right, ask to move the right bound
		proposedRightBorder := goal + controller.SafetyMargin
		success := controller.requestBoundMove(proposedRightBorder, controller.OutgoingRightRequest, controller.IncomingRightResponse)
		if success {
			controller.RightBound = proposedRightBorder
			controller.acceptGoal(goal)
		} else {
			// if the right bound is rejected, reject the goal
			fmt.Println(controller.Cart.Name, ": Right bound rejected, goal rejected: ", goal)
			controller.State = Idle
		}
		return success
	} else {
		// accept the goal
		controller.acceptGoal(goal)
		return true
	}
}

func (controller *Controller) avoidCollision(newGoal float64, outgoingResponse chan<- Response) {
	// save the old goal
	controller.OldState = controller.State
	controller.OldGoal = controller.Goal

	// set the new goal
	if controller.processGoal(newGoal) {
		fmt.Println(controller.Cart.Name, ": Avoiding collision, new goal: ", newGoal)

		// set the state to avoiding
		controller.State = Avoiding

		// send a response to the other cart
		outgoingResponse <- ACCEPT // TODO: unsafe
	} else {
		fmt.Println(controller.Cart.Name, ": Cannot avoid collision, rejecting request: ", newGoal)

		outgoingResponse <- REJECT
	}
}

func clamp(value, min, max float64) float64 {
	return math.Min(math.Max(value, min), max)
}

func (controller *Controller) run_controller() {

	fmt.Println(controller.Cart.Name, ": Starting controller")

	// first accept the goal to initial position (to initialize the MPC)
	controller.acceptGoal(controller.Cart.Position)

	// Run the PIDs in a separate goroutine
	go func() {
		ticker := time.NewTicker(time.Second / 100)
		defer ticker.Stop()
		for range ticker.C {
			controller.PositionPID.SetSetpoint(controller.boundedPositionSetpoint())
			control_velocity := controller.PositionPID.Update(controller.Cart.Position)
			controller.VelocityPID.SetSetpoint(control_velocity)
			control_force := controller.VelocityPID.Update(controller.Cart.Velocity)

			controller.Cart.applyForce(control_force)
		}
	}()

	// handle incoming requests in a separate goroutine
	go func() {
		for {
			select {
			case request := <-controller.IncomingLeftRequest:
				proposedLeftBorder := request.proposed_border
				fmt.Println(controller.Cart.Name, ": Received request to move left bound to: ", proposedLeftBorder)
				if controller.Goal > proposedLeftBorder+controller.SafetyMargin {
					fmt.Println(controller.Cart.Name, ": Accepting request to move left bound to: ", proposedLeftBorder)
					controller.OutgoingLeftResponse <- Response(ACCEPT)
					controller.LeftBound = proposedLeftBorder
				} else {
					if controller.State == Avoiding {
						// if we are avoiding, we cannot accept the request
						controller.OutgoingLeftResponse <- Response(WAIT)
					} else if request.priority.Before(controller.GoalReceived) || controller.State == Idle {
						// if the other cart's goal is older than our goal, we yield right of way to it
						// otherwise we tell it to wait until we complete our goal
						controller.LeftBound = proposedLeftBorder // TODO: unsafe
						controller.avoidCollision(proposedLeftBorder+1.1*controller.SafetyMargin, controller.OutgoingLeftResponse)
					} else {
						controller.OutgoingLeftResponse <- Response(WAIT)
					}
				}
			case request := <-controller.IncomingRightRequest:
				proposedRightBorder := request.proposed_border
				fmt.Println(controller.Cart.Name, ": Received request to move right bound to: ", proposedRightBorder)
				if controller.Goal < proposedRightBorder-controller.SafetyMargin {
					fmt.Println(controller.Cart.Name, ": Accepting request to move right bound to: ", proposedRightBorder)
					controller.OutgoingRightResponse <- Response(ACCEPT)
					controller.RightBound = proposedRightBorder
				} else {
					if controller.State == Avoiding {
						// if we are avoiding, we cannot accept the request
						controller.OutgoingRightResponse <- Response(WAIT)
						fmt.Println(controller.Cart.Name, ": Rejecting request to move right bound to: ", proposedRightBorder)
					} else if request.priority.Before(controller.GoalReceived) || controller.State == Idle {
						// if the other cart's goal is older than our goal, we yield right of way to it
						// otherwise we tell it to wait until we complete our goal
						controller.RightBound = proposedRightBorder // TODO: unsafe
						controller.avoidCollision(proposedRightBorder-1.1*controller.SafetyMargin, controller.OutgoingRightResponse)
					} else {
						controller.OutgoingRightResponse <- Response(WAIT)
					}
				}
			}
		}
	}()

	ticker := time.NewTicker(time.Second / 100)
	defer ticker.Stop()

	// Run the controller in a loop
	for range ticker.C {
		switch controller.State {
		case Idle:
			controller.checkForNewGoal()
		case Moving:
			// check if the cart is close to the goal
			if math.Abs(controller.Cart.Position-controller.Goal) < 10 {
				// goal reached, go back to idle
				fmt.Println(controller.Cart.Name, ": Goal reached: ", controller.Goal)
				controller.State = Idle
			}
		case Avoiding:
			// check if we have moved out of the way
			if math.Abs(controller.Cart.Position-controller.Goal) < 10 {
				// goal reached, go back to old state
				if controller.OldState == Moving {
					controller.State = Moving
					controller.processGoal(controller.OldGoal)
				} else {
					controller.State = Idle
				}
			}
		}
	}
}
