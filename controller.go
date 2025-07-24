package main

import (
	"fmt"
	"time"
)

type State int

const (
	Idle State = iota
	Moving
	Avoiding
)

func (s State) String() string {
	switch s {
	case Idle:
		return "Idle"
	case Moving:
		return "Moving"
	case Avoiding:
		return "Avoiding"
	default:
		return "Unknown"
	}
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
		IncomingGoalRequest:   make(chan float64, 10), // Buffered to prevent blocking
		State:                 Idle,
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
	if c.LeftBorderTrajectory.end+c.safetyMargin < goal && goal < c.RightBorderTrajectory.end-c.safetyMargin {
		c.acceptGoal(goal)
	} else {
		c.rejectGoal(goal)
	}
}

func (c *Controller) acceptGoal(goal float64) {
	fmt.Println(c.Cart.Name, ": Goal accepted:", goal)
	// Handle incoming goal request
	trajectory := c.MovementPlanner.CalculateTrajectory(c.Cart.Position, goal)
	c.CurrentTrajectory = &trajectory
	c.State = Moving
}

func (c *Controller) rejectGoal(goal float64) {
	fmt.Println(c.Cart.Name, ": Goal permanently rejected:", goal)
}
