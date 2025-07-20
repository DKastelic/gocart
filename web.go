package main

import (
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
)

type ControlMessage struct {
	Command    string  `json:"command"`
	Controller int     `json:"controller,omitempty"`
	Position   float64 `json:"position,omitempty"`
	Enabled    bool    `json:"enabled,omitempty"`
}

type SocketData struct {
	Id           int     `json:"id"`
	Position     float64 `json:"position"`
	Velocity     float64 `json:"velocity"`
	Acceleration float64 `json:"acceleration"`
	Jerk         float64 `json:"jerk"`
	Timestamp    string  `json:"timestamp"`

	LeftBorder  float64 `json:"leftBorder"`
	RightBorder float64 `json:"rightBorder"`
	Goal        float64 `json:"goal"`
	Setpoint    float64 `json:"setpoint"`
	State       string  `json:"state"` // "Idle", "Moving", "Avoiding"
}

// Upgrader is used to upgrade HTTP connections to WebSocket connections.
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func wsHandler(w http.ResponseWriter, r *http.Request, c <-chan SocketData, controllerGoalChannels []chan<- float64, randomControlChannel chan<- ControlMessage) {
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

func startWebsocketServer(controllerDataChannels []chan SocketData, controllerGoalChannels []chan<- float64, randomControlChannel chan<- ControlMessage) {
	// Create a channel to aggregate data from all controllers
	aggregatedChannel := make(chan SocketData)

	// Start a goroutine to aggregate data from all controller channels
	go func() {
		for {
			for _, ch := range controllerDataChannels {
				select {
				case data := <-ch:
					aggregatedChannel <- data
				default:
					// No data available, continue
				}
			}
		}
	}()

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		wsHandler(w, r, aggregatedChannel, controllerGoalChannels, randomControlChannel)
	})

	fmt.Println("WebSocket server started on :8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}
