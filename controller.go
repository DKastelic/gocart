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
	State ControllerState
	Goal  float64

	// The cart's old goal while avoiding
	OldState ControllerState
	OldGoal  float64

	// Current bounds
	LeftBound    float64
	RightBound   float64
	SafetyMargin float64

	// Channels for communication with the neighboring carts
	IncomingLeftRequest   <-chan float64
	IncomingRightRequest  <-chan float64
	OutgoingLeftRequest   chan<- float64
	OutgoingRightRequest  chan<- float64
	IncomingLeftResponse  <-chan Response
	IncomingRightResponse <-chan Response
	OutgoingLeftResponse  chan<- Response
	OutgoingRightResponse chan<- Response
}

// NewController creates a new controller with the given PID controllers
func NewController(cart *Cart, leftBound, rightBound float64) *Controller {
	return &Controller{
		Cart:         cart,
		Goal:         cart.Position,
		VelocityPID:  NewPID(30, 0, 0, -10000.0, 10000.0, 0.01),
		PositionPID:  NewPID(5, 0, 0, -1000.0, 1000.0, 0.01),
		State:        Idle,
		LeftBound:    leftBound,
		RightBound:   rightBound,
		SafetyMargin: 30,
	}
}

// This could be eg. retrieved from a request
func getGoal() float64 {
	// Generate a random goal between 0 and 1000
	return rand.Float64()*1200 + 200
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

func (controller *Controller) requestBoundMove(proposedBorder float64, outgoingRequest chan<- float64, incomingResponse <-chan Response) bool {
	controller.State = Requesting

	fmt.Println(controller.Cart.Name, ": Requesting to move border to: ", proposedBorder)

	if outgoingRequest == nil {
		fmt.Println(controller.Cart.Name, ": Hard bound, rejecting request locally")
		return false
	}

	select {
	case outgoingRequest <- proposedBorder:
		response := <-incomingResponse
		switch response {
		case WAIT:
			d := rand.IntN(1000)
			fmt.Println(controller.Cart.Name, ": Waiting")
			time.Sleep(time.Duration(d) * time.Millisecond)
			controller.requestBoundMove(proposedBorder, outgoingRequest, incomingResponse)
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
			// real_force := math.Min(math.Max(control_force, -1000.0), 1000.0)
			// real_force = math.Min(math.Max(real_force, controller.Cart.Force-100), controller.Cart.Force+100)
			real_force := control_force // good enough for now

			controller.Cart.applyForce(real_force)
		}
	}()

	// handle incoming requests in a separate goroutine
	go func() {
		for {
			select {
			case proposedLeftBorder := <-controller.IncomingLeftRequest:
				fmt.Println(controller.Cart.Name, ": Received request to move left bound to: ", proposedLeftBorder)
				if controller.Cart.Position > proposedLeftBorder+controller.SafetyMargin && controller.Goal > proposedLeftBorder+controller.SafetyMargin {
					fmt.Println(controller.Cart.Name, ": Accepting request to move left bound to: ", proposedLeftBorder)
					controller.OutgoingLeftResponse <- Response(ACCEPT)
					controller.LeftBound = proposedLeftBorder
				} else {
					controller.avoidCollision(proposedLeftBorder+2*controller.SafetyMargin, controller.OutgoingLeftResponse)
				}
			case proposedRightBorder := <-controller.IncomingRightRequest:
				fmt.Println(controller.Cart.Name, ": Received request to move right bound to: ", proposedRightBorder)
				if controller.Cart.Position < proposedRightBorder-controller.SafetyMargin && controller.Goal < proposedRightBorder-controller.SafetyMargin {
					fmt.Println(controller.Cart.Name, ": Accepting request to move right bound to: ", proposedRightBorder)
					controller.OutgoingRightResponse <- Response(ACCEPT)
					controller.RightBound = proposedRightBorder
				} else {
					controller.avoidCollision(proposedRightBorder-2*controller.SafetyMargin, controller.OutgoingRightResponse)
				}
			}
		}
	}()

	ticker := time.NewTicker(time.Second / 1)
	defer ticker.Stop()

	// Run the controller in a loop
	for range ticker.C {

		// fmt.Println(controller.Cart.Name, ": Position: ", controller.Cart.Position, " Goal: ", controller.Goal, " State: ", controller.State)

		switch controller.State {
		case Idle:
			goal := getGoal()
			controller.processGoal(goal)
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
