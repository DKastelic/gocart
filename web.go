package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
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

// HistoricalDataStore manages historical data storage with automatic cleanup
type HistoricalDataStore struct {
	mu       sync.RWMutex
	data     []AllCartsData
	maxAge   time.Duration
	maxCount int
}

// NewHistoricalDataStore creates a new historical data store
func NewHistoricalDataStore(maxAge time.Duration, maxCount int) *HistoricalDataStore {
	return &HistoricalDataStore{
		data:     make([]AllCartsData, 0, maxCount),
		maxAge:   maxAge,
		maxCount: maxCount,
	}
}

// AddDataPoint adds a new data point and manages cleanup
func (hds *HistoricalDataStore) AddDataPoint(allCartsData AllCartsData) {
	hds.mu.Lock()
	defer hds.mu.Unlock()

	hds.data = append(hds.data, allCartsData)

	// Parse timestamp for age-based cleanup
	timestamp, err := time.Parse(time.RFC3339Nano, allCartsData.Timestamp)
	if err != nil {
		// If timestamp parsing fails, use current time
		timestamp = time.Now()
	}

	// Remove old data by age
	cutoff := timestamp.Add(-hds.maxAge)
	start := 0
	for i, data := range hds.data {
		if dataTimestamp, err := time.Parse(time.RFC3339Nano, data.Timestamp); err == nil && dataTimestamp.After(cutoff) {
			start = i
			break
		}
	}
	if start > 0 {
		hds.data = hds.data[start:]
	}

	// Remove old data by count
	if len(hds.data) > hds.maxCount {
		excess := len(hds.data) - hds.maxCount
		hds.data = hds.data[excess:]
	}
}

// GetHistoricalData returns all historical data points
func (hds *HistoricalDataStore) GetHistoricalData() []AllCartsData {
	hds.mu.RLock()
	defer hds.mu.RUnlock()

	result := make([]AllCartsData, len(hds.data))
	copy(result, hds.data)
	return result
}

// Clear removes all historical data (used on fresh connections)
func (hds *HistoricalDataStore) Clear() {
	hds.mu.Lock()
	defer hds.mu.Unlock()
	hds.data = hds.data[:0]
}

// Global historical data store
var historicalStore *HistoricalDataStore

// Upgrader is used to upgrade HTTP connections to WebSocket connections.
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func wsHandler(w http.ResponseWriter, r *http.Request, c <-chan AllCartsData, controllerGoalChannels []chan<- float64, randomControlChannel chan<- ControlMessage) {
	// Clear historical data on fresh connection
	historicalStore.Clear()

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

// historicalDataHandler serves historical data as JSON
func historicalDataHandler(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	// Handle preflight requests
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	data := historicalStore.GetHistoricalData()

	if err := json.NewEncoder(w).Encode(data); err != nil {
		fmt.Printf("Error encoding historical data: %v\n", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// collectCartData collects the current data from a controller
func collectCartData(controller *Controller) SocketData {
	return SocketData{
		Id: controller.Cart.Id,

		// Chart data (planned trajectory values from MovementPlanner)
		ChartPosition:     controller.CurrentTrajectory.GetCurrentPosition(),
		ChartVelocity:     controller.CurrentTrajectory.GetCurrentVelocity(),
		ChartAcceleration: controller.CurrentTrajectory.GetCurrentAcceleration(),
		ChartJerk:         controller.CurrentTrajectory.GetCurrentJerk(),

		// Real-time data (actual cart physics values)
		Position: controller.Cart.Position,

		LeftBorder:  controller.LeftBorderTrajectory.GetCurrentPosition(),
		RightBorder: controller.RightBorderTrajectory.GetCurrentPosition(),
		Goal:        controller.CurrentTrajectory.end,
		Setpoint:    controller.PositionPID.Setpoint,
		State:       controller.State.String(),
	}
}

func startWebsocketServer(controllers []*Controller, controllerGoalChannels []chan<- float64, randomControlChannel chan<- ControlMessage) {
	// Initialize the historical data store (keep 5 minutes of data at 30Hz = 9000 points max)
	historicalStore = NewHistoricalDataStore(5*time.Minute, 9000)

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

			// Store data point in historical store
			historicalStore.AddDataPoint(allCartsData)

			select {
			case dataChannel <- allCartsData:
				// Data sent successfully
			default:
				// Drop if channel is full
			}
		}
	}()

	// Register HTTP handlers
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		wsHandler(w, r, dataChannel, controllerGoalChannels, randomControlChannel)
	})
	http.HandleFunc("/api/historical-data", historicalDataHandler)

	fmt.Println("WebSocket server started on :8080")
	fmt.Println("Historical data API available at :8080/api/historical-data")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}
