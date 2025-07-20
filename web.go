package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

type ControlMessage struct {
	Command    string  `json:"command"`
	Controller int     `json:"controller,omitempty"`
	Position   float64 `json:"position,omitempty"`
	Enabled    bool    `json:"enabled,omitempty"`
}

type SocketData struct {
	Id int `json:"id"`

	// Chart data (planned trajectory values from MovementPlanner)
	ChartPosition     float64 `json:"chartPosition"`
	ChartVelocity     float64 `json:"chartVelocity"`
	ChartAcceleration float64 `json:"chartAcceleration"`
	ChartJerk         float64 `json:"chartJerk"`

	// Real-time data (actual cart physics values)
	Position float64 `json:"position"`

	LeftBorder  float64 `json:"leftBorder"`
	RightBorder float64 `json:"rightBorder"`
	Goal        float64 `json:"goal"`
	Setpoint    float64 `json:"setpoint"`
	State       string  `json:"state"` // "Idle", "Moving", "Avoiding"
}

type AllCartsData struct {
	Carts     []SocketData `json:"carts"`
	Timestamp string       `json:"timestamp"`
}

// Upgrader is used to upgrade HTTP connections to WebSocket connections.
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func wsHandler(w http.ResponseWriter, r *http.Request, c <-chan AllCartsData, controllerGoalChannels []chan<- float64, randomControlChannel chan<- ControlMessage) {
	// Upgrade the HTTP connection to a WebSocket connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("Error upgrading:", err)
		return
	}
	defer conn.Close()

	// Start a goroutine to handle incoming messages from the client
	go func() {
		for {
			var msg ControlMessage
			err := conn.ReadJSON(&msg)
			if err != nil {
				fmt.Println("Error reading from WebSocket:", err)
				return
			}

			// Process the control message
			switch msg.Command {
			case "setGoal":
				if msg.Controller >= 0 && msg.Controller < len(controllerGoalChannels) {
					fmt.Printf("Frontend: Setting goal for cart %d to %f\n", msg.Controller+1, msg.Position)
					controllerGoalChannels[msg.Controller] <- msg.Position
				}
			case "randomGoals":
				fmt.Printf("Frontend: %s random goal generation\n", map[bool]string{true: "Starting", false: "Stopping"}[msg.Enabled])
				randomControlChannel <- msg
			}
		}
	}()

	// send every value from the provided channel to the WebSocket client
	for value := range c {
		err := conn.WriteJSON(value)
		if err != nil {
			fmt.Println("Error writing to WebSocket:", err)
			return
		}
	}
}

// collectCartData collects the current data from a controller
func collectCartData(controller *Controller) SocketData {
	return SocketData{
		Id: controller.Cart.Id,

		// Chart data (planned trajectory values from MovementPlanner)
		ChartPosition:     controller.MovementPlanner.GetCurrentTrajectoryPosition(controller.MPCTrajectory),
		ChartVelocity:     controller.MovementPlanner.GetCurrentTrajectoryVelocity(controller.MPCTrajectory),
		ChartAcceleration: controller.MovementPlanner.GetCurrentTrajectoryAcceleration(controller.MPCTrajectory),
		ChartJerk:         controller.MovementPlanner.GetCurrentTrajectoryJerk(controller.MPCTrajectory),

		// Real-time data (actual cart physics values)
		Position: controller.Cart.Position,

		LeftBorder:  controller.LeftBound,
		RightBorder: controller.RightBound,
		Goal:        controller.Goal,
		Setpoint:    controller.PositionPID.Setpoint,
		State:       controller.State.String(),
	}
}

func startWebsocketServer(controllers []*Controller, controllerGoalChannels []chan<- float64, randomControlChannel chan<- ControlMessage) {
	// Create a channel for broadcasting data
	dataChannel := make(chan AllCartsData, 100)

	// Start data broadcasting goroutine
	go func() {
		ticker := time.NewTicker(time.Second / 30) // Send data 30 times per second
		defer ticker.Stop()

		for range ticker.C {
			// Collect data for all controllers in a single message
			var cartsData []SocketData
			timestamp := time.Now().UTC().Format(time.RFC3339Nano)

			for _, controller := range controllers {
				data := collectCartData(controller)
				cartsData = append(cartsData, data)
			}

			allCartsData := AllCartsData{
				Carts:     cartsData,
				Timestamp: timestamp,
			}

			select {
			case dataChannel <- allCartsData:
				// Data sent successfully
			default:
				// Drop if channel is full
			}
		}
	}()

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		wsHandler(w, r, dataChannel, controllerGoalChannels, randomControlChannel)
	})

	fmt.Println("WebSocket server started on :8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}
