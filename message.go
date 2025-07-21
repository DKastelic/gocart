package main

import "time"

type Response int

const (
	ACCEPT Response = iota
	REJECT
	WAIT
	GOAL_REACHED
)

type Request struct {
	proposed_border Trajectory
	priority        time.Time
}

func connectControllers(leftController, rightController *Controller) {
	// give them 1 capacity to make them non-blocking
	// this is to avoid deadlocks when one controller is waiting for the other
	// to respond to a request
	leftToRightRequest := make(chan Request)
	rightToLeftRequest := make(chan Request)
	leftToRightResponse := make(chan Response)
	rightToLeftResponse := make(chan Response)
	leftController.OutgoingRightRequest = leftToRightRequest
	leftController.IncomingRightRequest = rightToLeftRequest
	leftController.OutgoingRightResponse = leftToRightResponse
	leftController.IncomingRightResponse = rightToLeftResponse
	rightController.OutgoingLeftRequest = rightToLeftRequest
	rightController.IncomingLeftRequest = leftToRightRequest
	rightController.OutgoingLeftResponse = rightToLeftResponse
	rightController.IncomingLeftResponse = leftToRightResponse
}
