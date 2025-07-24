package main

func main() {

	// // Initialize carts
	// carts := []Cart{
	// 	{Name: "Cart 1", Id: 1, Position: 200, Velocity: 0, Acceleration: 0, Mass: 1, Force: 0, Width: 50, Height: 40},
	// 	{Name: "Cart 2", Id: 2, Position: 600, Velocity: 0, Acceleration: 0, Mass: 1, Force: 0, Width: 50, Height: 40},
	// 	{Name: "Cart 3", Id: 3, Position: 1000, Velocity: 0, Acceleration: 0, Mass: 1, Force: 0, Width: 50, Height: 40},
	// 	{Name: "Cart 4", Id: 4, Position: 1400, Velocity: 0, Acceleration: 0, Mass: 1, Force: 0, Width: 50, Height: 40},
	// }

	// // Initialize the cart controllers
	// controllers := []*Controller{
	// 	NewController(&carts[0], 0, 400),
	// 	NewController(&carts[1], 400, 800),
	// 	NewController(&carts[2], 800, 1200),
	// 	NewController(&carts[3], 1200, 1600),
	// }
	carts := []Cart{
		{Name: "Cart 1", Id: 1, Position: 400, Velocity: 0, Acceleration: 0, Mass: 1, Force: 0, Width: 50, Height: 40},
		{Name: "Cart 2", Id: 2, Position: 1200, Velocity: 0, Acceleration: 0, Mass: 1, Force: 0, Width: 50, Height: 40},
	}
	controllers := []*Controller{
		NewController(&carts[0], 0, 800),
		NewController(&carts[1], 800, 1600),
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

	// Create and start the goal manager
	goalManager := NewGoalManager(controllerGoalChannels, randomControlChannel)
	goalManager.Start()

	go input_loop(controllerGoalChannels, exit_channel, randomControlChannel)

	// Start the controllers
	for i := range controllers {
		go controllers[i].run_controller()
	}

	// Initialize the WebSocket server with controllers
	startWebsocketServer(controllers, controllerGoalChannels, randomControlChannel)

	// wait for the exit signal
	<-exit_channel
}
