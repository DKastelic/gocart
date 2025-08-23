package main

type ResponseType int

const (
	ACCEPT ResponseType = iota
	REJECT
	WAIT
	STOP_CONFIRM
)

type Response struct {
	RequestId int64
	Type      ResponseType
}

type RequestType int

const (
	BORDER_MOVE RequestType = iota
	EMERGENCY_STOP
)

type Request struct {
	RequestId           int64
	Type                RequestType
	ProposedBorderStart float64
	ProposedBorderEnd   float64
}
