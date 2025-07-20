package main

import (
	"fmt"
	"math/rand"
)

func handleRandomGoalGeneration(controllerGoalChannels []chan<- float64, randomControlChannel <-chan ControlMessage) {
	var stopChannels = make([]chan struct{}, len(controllerGoalChannels))

	for msg := range randomControlChannel {
		if msg.Command == "randomGoals" {
			if msg.Enabled {
				fmt.Println("Starting automatic goal generation...")
				for i, ch := range controllerGoalChannels {
					stopChannels[i] = make(chan struct{})
					go func(index int, channel chan<- float64, stop chan struct{}) {
						for {
							select {
							case <-stop:
								return
							default:
								goal := rand.Float64()*1200 + 200 // Random goal between 200 and 1400
								channel <- goal
								fmt.Printf("Generated goal for controller %d: %f\n", index, goal)
							}
						}
					}(i, ch, stopChannels[i])
				}
			} else {
				fmt.Println("Stopping automatic goal generation...")
				for _, stop := range stopChannels {
					stop <- struct{}{}
				}
			}
		}
	}
}

func main() {

	// Initialize carts
	carts := []Cart{
		{Name: "Cart 1", Id: 1, Position: 200, Velocity: 0, Acceleration: 0, Mass: 1, Force: 0, Width: 50, Height: 40},
		{Name: "Cart 2", Id: 2, Position: 900, Velocity: 0, Acceleration: 0, Mass: 1, Force: 0, Width: 50, Height: 40},
		// {Name: "Cart 3", Position: 1000, Velocity: 0, Acceleration: 0, Mass: 1, Force: 0, Width: 50, Height: 40},
		// {Name: "Cart 4", Position: 1400, Velocity: 0, Acceleration: 0, Mass: 1, Force: 0, Width: 50, Height: 40},
	}

	// Initialize the cart controllers
	controllers := []*Controller{
		NewController(&carts[0], 0, 800),
		NewController(&carts[1], 800, 1600),
		// NewController(&carts[2], 800, 1200),
		// NewController(&carts[3], 1200, 1600),
	}

	for i := 0; i < len(controllers)-1; i++ {
		connectControllers(controllers[i], controllers[i+1])
	}

	// Initialize channels for controller goals
	controllerGoalChannels := []chan<- float64{}
	for i := range controllers {
		ch := make(chan float64)
		controllers[i].IncomingGoalRequest = ch
		controllerGoalChannels = append(controllerGoalChannels, ch)
	}

	// Initialize exit channel
	exit_channel := make(chan struct{})
	defer close(exit_channel)

	// Start the physics and drawing loops
	go physics_loop(carts)
	// go draw_loop(controllers, exit_channel)
	// Create a channel for random goal control from the frontend
	randomControlChannel := make(chan ControlMessage, 10)

	// Handle random goal generation
	go handleRandomGoalGeneration(controllerGoalChannels, randomControlChannel)

	go input_loop(controllerGoalChannels, exit_channel)

	// And channels for controllers to send data to the WebSocket server
	controllerDataChannels := []chan SocketData{}
	for i := range controllers {
		ch := make(chan SocketData, 1)
		controllers[i].OutgoingData = ch
		controllerDataChannels = append(controllerDataChannels, ch)
	}

	// Start the controllers
	for i := range controllers {
		go controllers[i].run_controller()
	}

	// Initialize the WebSocket server with the controller data channels
	startWebsocketServer(controllerDataChannels, controllerGoalChannels, randomControlChannel)

	// wait for the exit signal
	<-exit_channel
}
