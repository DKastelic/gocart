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

type Controller struct {
	// provide direct access to the cart's physical properties
	// "intrinsic sensing and perfect actuation", good enough for our purposes
	Cart *Cart

	// PID controllers for velocity and position
	VelocityPID *PID
	PositionPID *PID

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

	// Channel for sending data to the WebSocket server
	OutgoingData chan<- SocketData
}

// NewController creates a new controller with the given PID controllers
func NewController(cart *Cart, leftBound, rightBound float64) *Controller {
	return &Controller{
		Cart:         cart,
		Goal:         cart.Position,
		VelocityPID:  NewPID(20, 0, 0, -10000.0, 10000.0, 0.01),
		PositionPID:  NewPID(3, 0, 0, -1000.0, 1000.0, 0.01),
		State:        Idle,
		LeftBound:    leftBound,
		RightBound:   rightBound,
		SafetyMargin: 30,
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
}

func (controller *Controller) boundedPositionSetpoint() float64 {
	return math.Min(math.Max(controller.Goal, controller.LeftBound+controller.SafetyMargin), controller.RightBound-controller.SafetyMargin)
}

func (controller *Controller) requestBoundMove(proposedBorder float64, outgoingRequest chan<- Request, incomingResponse <-chan Response) bool {
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
	case <-time.After(100 * time.Millisecond):
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
		outgoingResponse <- WAIT
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

	// Run the PIDs in a separate goroutine
	go func() {
		ticker := time.NewTicker(time.Second / 100)
		defer ticker.Stop()
		for range ticker.C {
			controller.PositionPID.SetSetpoint(controller.boundedPositionSetpoint())
			control_velocity := controller.PositionPID.Update(controller.Cart.Position)
			controller.VelocityPID.SetSetpoint(control_velocity)
			control_force := controller.VelocityPID.Update(controller.Cart.Velocity)

			// limit the force to a reasonable range, and a reasonable deviation from the previous force
			real_force := clamp(control_force, -10000.0, 10000.0)
			real_force = clamp(real_force, controller.Cart.Force-500, controller.Cart.Force+500)
			// real_force := control_force

			controller.Cart.applyForce(real_force)
		}
	}()

	// Send the cart data to the frontend 10 times a second
	go func() {
		ticker := time.NewTicker(time.Second / 10)
		defer ticker.Stop()
		for range ticker.C {
			// send the cart's data to the WebSocket server
			select {
			case controller.OutgoingData <- SocketData{
				Id:           controller.Cart.Id,
				Position:     controller.Cart.Position,
				Velocity:     controller.Cart.Velocity,
				Acceleration: controller.Cart.Acceleration,
				Jerk:         0, // TODO
			}:
			default:
				// drop if channel is full
			}
		}
	}()

	// handle incoming requests in a separate goroutine
	go func() {
		for {
			select {
			case request := <-controller.IncomingLeftRequest:
				proposedLeftBorder := request.proposed_border
				fmt.Println(controller.Cart.Name, ": Received request to move left bound to: ", proposedLeftBorder)
				if controller.Cart.Position > proposedLeftBorder+controller.SafetyMargin && controller.Goal > proposedLeftBorder+controller.SafetyMargin {
					fmt.Println(controller.Cart.Name, ": Accepting request to move left bound to: ", proposedLeftBorder)
					controller.OutgoingLeftResponse <- Response(ACCEPT)
					controller.LeftBound = proposedLeftBorder
				} else {
					// if the other cart's goal is older than our goal, we yield right of way to it
					// otherwise we tell it to wait until we complete our goal
					if request.priority.Before(controller.GoalReceived) {
						controller.avoidCollision(proposedLeftBorder+1.1*controller.SafetyMargin, controller.OutgoingLeftResponse)
					} else {
						controller.OutgoingLeftResponse <- Response(WAIT)
					}
				}
			case request := <-controller.IncomingRightRequest:
				proposedRightBorder := request.proposed_border
				fmt.Println(controller.Cart.Name, ": Received request to move right bound to: ", proposedRightBorder)
				if controller.Cart.Position < proposedRightBorder-controller.SafetyMargin && controller.Goal < proposedRightBorder-controller.SafetyMargin {
					fmt.Println(controller.Cart.Name, ": Accepting request to move right bound to: ", proposedRightBorder)
					controller.OutgoingRightResponse <- Response(ACCEPT)
					controller.RightBound = proposedRightBorder
				} else {
					// if the other cart's goal is older than our goal, we yield right of way to it
					// otherwise we tell it to wait until we complete our goal
					if request.priority.Before(controller.GoalReceived) {
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

		// fmt.Println(controller.Cart.Name, ": Position: ", controller.Cart.Position, " Goal: ", controller.Goal, " State: ", controller.State)

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
