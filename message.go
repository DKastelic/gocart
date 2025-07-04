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
	proposed_border float64
	priority        time.Time
}

func connectControllers(leftController, rightController *Controller) {
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
