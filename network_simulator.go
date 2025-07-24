package main

import (
	"math/rand/v2"
	"time"
)

// NetworkSimulator simulates network delays for inter-controller communication
type NetworkSimulator struct {
	MinDelay time.Duration
	MaxDelay time.Duration
}

// DefaultNetworkSimulator creates a network simulator with default delays (10-50ms)
func DefaultNetworkSimulator() *NetworkSimulator {
	return NewNetworkSimulator(10*time.Millisecond, 50*time.Millisecond)
}

// NewNetworkSimulator creates a new network simulator with specified delay range
func NewNetworkSimulator(minDelay, maxDelay time.Duration) *NetworkSimulator {
	return &NetworkSimulator{
		MinDelay: minDelay,
		MaxDelay: maxDelay,
	}
}

// getRandomDelay returns a random delay within the configured range
func (ns *NetworkSimulator) getRandomDelay() time.Duration {
	if ns.MinDelay >= ns.MaxDelay {
		return ns.MinDelay
	}
	delayRange := int64(ns.MaxDelay - ns.MinDelay)
	randomDelay := time.Duration(rand.Int64N(delayRange))
	return ns.MinDelay + randomDelay
}

// DelayedRequestChannel creates a delayed channel for Request messages
func (ns *NetworkSimulator) DelayedRequestChannel(input <-chan Request, output chan<- Request) {
	go func() {
		for req := range input {
			delay := ns.getRandomDelay()
			go func(request Request, d time.Duration) {
				time.Sleep(d)
				output <- request
			}(req, delay)
		}
		close(output)
	}()
}

// DelayedResponseChannel creates a delayed channel for Response messages
func (ns *NetworkSimulator) DelayedResponseChannel(input <-chan Response, output chan<- Response) {
	go func() {
		for resp := range input {
			delay := ns.getRandomDelay()
			go func(response Response, d time.Duration) {
				time.Sleep(d)
				output <- response
			}(resp, delay)
		}
		close(output)
	}()
}

// DelayedChannelPair creates a pair of delayed channels (request and response) between two controllers
type DelayedChannelPair struct {
	// Channels for controller A to B communication
	AtoB_RequestInput   chan<- Request
	AtoB_RequestOutput  <-chan Request
	AtoB_ResponseInput  chan<- Response
	AtoB_ResponseOutput <-chan Response

	// Channels for controller B to A communication
	BtoA_RequestInput   chan<- Request
	BtoA_RequestOutput  <-chan Request
	BtoA_ResponseInput  chan<- Response
	BtoA_ResponseOutput <-chan Response
}

// NewDelayedChannelPair creates a new pair of delayed channels with network simulation
func NewDelayedChannelPair(simulator *NetworkSimulator) *DelayedChannelPair {
	// Create intermediate channels
	aToB_RequestBuffer := make(chan Request, 10)
	aToB_RequestDelayed := make(chan Request, 10)
	aToB_ResponseBuffer := make(chan Response, 10)
	aToB_ResponseDelayed := make(chan Response, 10)

	bToA_RequestBuffer := make(chan Request, 10)
	bToA_RequestDelayed := make(chan Request, 10)
	bToA_ResponseBuffer := make(chan Response, 10)
	bToA_ResponseDelayed := make(chan Response, 10)

	// Set up delay simulators
	simulator.DelayedRequestChannel(aToB_RequestBuffer, aToB_RequestDelayed)
	simulator.DelayedResponseChannel(aToB_ResponseBuffer, aToB_ResponseDelayed)
	simulator.DelayedRequestChannel(bToA_RequestBuffer, bToA_RequestDelayed)
	simulator.DelayedResponseChannel(bToA_ResponseBuffer, bToA_ResponseDelayed)

	return &DelayedChannelPair{
		AtoB_RequestInput:   aToB_RequestBuffer,
		AtoB_RequestOutput:  aToB_RequestDelayed,
		AtoB_ResponseInput:  aToB_ResponseBuffer,
		AtoB_ResponseOutput: aToB_ResponseDelayed,

		BtoA_RequestInput:   bToA_RequestBuffer,
		BtoA_RequestOutput:  bToA_RequestDelayed,
		BtoA_ResponseInput:  bToA_ResponseBuffer,
		BtoA_ResponseOutput: bToA_ResponseDelayed,
	}
}
