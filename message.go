package main

type ResponseType int

const (
	ACCEPT ResponseType = iota
	REJECT
	WAIT
)

type Response struct {
	RequestId int64
	Type      ResponseType
}

type Request struct {
	RequestId           int64
	ProposedBorderStart float64
	ProposedBorderEnd   float64
}

func connectControllers(leftController, rightController *Controller) {
	// give them capacity to make them non-blocking
	// this is to avoid deadlocks when one controller is waiting for the other
	// to respond to a request
	leftToRightRequest := make(chan Request, 10)
	rightToLeftRequest := make(chan Request, 10)
	leftToRightResponse := make(chan Response, 10)
	rightToLeftResponse := make(chan Response, 10)
	leftController.OutgoingRightRequest = leftToRightRequest
	leftController.IncomingRightRequest = rightToLeftRequest
	leftController.OutgoingRightResponse = leftToRightResponse
	leftController.IncomingRightResponse = rightToLeftResponse
	rightController.OutgoingLeftRequest = rightToLeftRequest
	rightController.IncomingLeftRequest = leftToRightRequest
	rightController.OutgoingLeftResponse = rightToLeftResponse
	rightController.IncomingLeftResponse = leftToRightResponse
}
