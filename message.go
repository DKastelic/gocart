package main

type Response int

const (
	ACCEPT Response = iota
	REJECT
	WAIT
)

func connectControllers(leftController, rightController *Controller) {
	leftToRightRequest := make(chan float64)
	rightToLeftRequest := make(chan float64)
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
