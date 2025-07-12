package main

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
	startWebsocketServer(controllerDataChannels)

	// wait for the exit signal
	<-exit_channel
}
