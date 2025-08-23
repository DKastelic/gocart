package main

import (
	"fmt"
	"math/rand/v2"
	"time"
)

// NetworkDelaySimulator simulates network delays for requests and responses
type NetworkDelaySimulator struct {
	minDelay        time.Duration
	maxDelay        time.Duration
	lossProbability float64 // Probability of packet loss (0.0 to 1.0)
}

// NewNetworkDelaySimulator creates a new network intermediary with specified delay range
func NewNetworkDelaySimulator(minDelay, maxDelay time.Duration, lossProbability float64) *NetworkDelaySimulator {
	return &NetworkDelaySimulator{
		minDelay:        minDelay,
		maxDelay:        maxDelay,
		lossProbability: lossProbability,
	}
}

// getRandomDelay returns a random delay within the configured range
func (n *NetworkDelaySimulator) getRandomDelay() time.Duration {
	if n.maxDelay <= n.minDelay {
		return n.minDelay
	}
	delayRange := n.maxDelay - n.minDelay
	randomDelay := time.Duration(rand.Int64N(int64(delayRange)))
	return n.minDelay + randomDelay
}

// relayRequests relays requests from input to output with random delays
func (n *NetworkDelaySimulator) relayRequests(input <-chan Request, output chan<- Request) {
	go func() {
		for request := range input {
			delay := n.getRandomDelay()
			lossProbability := n.lossProbability
			go func(req Request, d time.Duration) {
				time.Sleep(d)

				// Simulate packet loss by randomly dropping requests
				if rand.Float64() < lossProbability {
					fmt.Print("Request dropped due to simulated packet loss\n")
					return
				}

				select {
				case output <- req:
					// Successfully forwarded
				default:
					// Output channel full, drop the request
				}
			}(request, delay)
		}
	}()
}

// relayResponses relays responses from input to output with random delays
func (n *NetworkDelaySimulator) relayResponses(input <-chan Response, output chan<- Response) {
	go func() {
		for response := range input {
			delay := n.getRandomDelay()
			lossProbability := n.lossProbability
			go func(resp Response, d time.Duration) {
				time.Sleep(d)

				// Simulate packet loss by randomly dropping responses
				if rand.Float64() < lossProbability {
					fmt.Print("Response dropped due to simulated packet loss\n")
					return
				}

				select {
				case output <- resp:
					// Successfully forwarded
				default:
					// Output channel full, drop the response
				}
			}(response, delay)
		}
	}()
}

func connectControllers(leftController, rightController *Controller) {
	// Create network intermediaries with 10-50ms delay range
	networkSim := NewNetworkDelaySimulator(10*time.Millisecond, 15*time.Millisecond, 0)

	// Create intermediate channels for the network simulation
	leftToRightRequestIntermediate := make(chan Request, 10)
	rightToLeftRequestIntermediate := make(chan Request, 10)
	leftToRightResponseIntermediate := make(chan Response, 10)
	rightToLeftResponseIntermediate := make(chan Response, 10)

	// Create the final channels that connect to controllers
	leftToRightRequest := make(chan Request, 10)
	rightToLeftRequest := make(chan Request, 10)
	leftToRightResponse := make(chan Response, 10)
	rightToLeftResponse := make(chan Response, 10)

	// Connect controllers to intermediate channels
	leftController.OutgoingRightRequest = leftToRightRequestIntermediate
	leftController.IncomingRightRequest = rightToLeftRequest
	leftController.OutgoingRightResponse = leftToRightResponseIntermediate
	leftController.IncomingRightResponse = rightToLeftResponse

	rightController.OutgoingLeftRequest = rightToLeftRequestIntermediate
	rightController.IncomingLeftRequest = leftToRightRequest
	rightController.OutgoingLeftResponse = rightToLeftResponseIntermediate
	rightController.IncomingLeftResponse = leftToRightResponse

	// Set up network intermediaries to relay with delays
	networkSim.relayRequests(leftToRightRequestIntermediate, leftToRightRequest)
	networkSim.relayRequests(rightToLeftRequestIntermediate, rightToLeftRequest)
	networkSim.relayResponses(leftToRightResponseIntermediate, leftToRightResponse)
	networkSim.relayResponses(rightToLeftResponseIntermediate, rightToLeftResponse)
}
