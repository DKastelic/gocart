package main

import (
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
)

type SocketData struct {
	Id           int     `json:"id"`
	Position     float64 `json:"position"`
	Velocity     float64 `json:"velocity"`
	Acceleration float64 `json:"acceleration"`
	Jerk         float64 `json:"jerk"`
}

// Upgrader is used to upgrade HTTP connections to WebSocket connections.
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func wsHandler(w http.ResponseWriter, r *http.Request, c <-chan SocketData) {
	// Upgrade the HTTP connection to a WebSocket connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("Error upgrading:", err)
		return
	}
	defer conn.Close()

	// send every value from the provided channel to the WebSocket client
	for value := range c {
		err := conn.WriteJSON(value)
		if err != nil {
			fmt.Println("Error writing to WebSocket:", err)
			return
		}
	}
}

func startWebsocketServer(controllerDataChannels []chan SocketData) {
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
		wsHandler(w, r, aggregatedChannel)
	})

	fmt.Println("WebSocket server started on :8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}
