package main

import "log"

func main() {

	// Initialize base cart definitions (these will be used as templates by the scenario manager)
	carts := []Cart{
		{Name: "Cart 1", Id: 1, Position: 200, Velocity: 0, Acceleration: 0, Mass: 1, Force: 0, Width: 50, Height: 40},
		{Name: "Cart 2", Id: 2, Position: 600, Velocity: 0, Acceleration: 0, Mass: 1, Force: 0, Width: 50, Height: 40},
		{Name: "Cart 3", Id: 3, Position: 1000, Velocity: 0, Acceleration: 0, Mass: 1, Force: 0, Width: 50, Height: 40},
		{Name: "Cart 4", Id: 4, Position: 1400, Velocity: 0, Acceleration: 0, Mass: 1, Force: 0, Width: 50, Height: 40},
	}

	// Initialize exit channel
	exit_channel := make(chan struct{})
	defer close(exit_channel)

	// Create a channel for random goal control from the frontend
	randomControlChannel := make(chan ControlMessage, 10)

	// Create the scenario manager with empty initial state (it will set up controllers when running default scenario)
	scenarioManager := NewScenarioManager(nil, nil, nil, carts, randomControlChannel, nil)

	// Start the physics loop with scenario manager
	go physics_loop(scenarioManager, exit_channel)

	// Start input loop
	go input_loop(scenarioManager, exit_channel, randomControlChannel)

	// Initialize the WebSocket server with scenario manager
	go func() {
		startWebsocketServer(scenarioManager)
	}()

	// Run the default scenario to set up the initial 4-cart configuration
	go func() {
		err := scenarioManager.RunScenario("Privzeti scenarij")
		if err != nil {
			log.Printf("Error running default scenario: %v", err)
		}
	}()

	// wait for the exit signal
	<-exit_channel
}
