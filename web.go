package main

import (
	"encoding/json"
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

type TestMessage struct {
	Type string      `json:"type"` // "start_tests", "test_status", "test_results"
	Data interface{} `json:"data,omitempty"`
}

type ScenarioMessage struct {
	Type string      `json:"type"` // "list_scenarios", "run_scenario", "scenario_status"
	Data interface{} `json:"data,omitempty"`
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

	// Trajectory phase transitions (timestamps when trajectory phases change)
	TrajectoryTransitions []string `json:"trajectoryTransitions"`

	// Trajectory phase information (phase labels corresponding to transitions)
	TrajectoryPhases []string `json:"trajectoryPhases"`

	// Performance metrics
	Metrics MessageMetricsReport `json:"metrics"`
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

func handleScenarioMessage(msgType string, rawMsg map[string]interface{}, scenarioManager *ScenarioManager, responseChannel chan<- ScenarioMessage) {
	switch msgType {
	case "list_scenarios":
		// Send list of available scenarios
		scenarios := scenarioManager.GetScenarios()
		response := ScenarioMessage{
			Type: "scenario_list",
			Data: scenarios,
		}
		responseChannel <- response

	case "run_scenario":
		// Run a specific scenario
		if scenarioName, ok := rawMsg["scenario"].(string); ok {
			go func() {
				err := scenarioManager.RunScenario(scenarioName)
				status := "completed"
				if err != nil {
					status = "failed"
				}

				response := ScenarioMessage{
					Type: "scenario_result",
					Data: map[string]interface{}{
						"scenario": scenarioName,
						"status":   status,
						"error":    err,
					},
				}
				responseChannel <- response
			}()
		}

	case "scenario_status":
		// Send current scenario statuses
		scenarios := scenarioManager.GetScenarios()
		response := ScenarioMessage{
			Type: "scenario_list",
			Data: scenarios,
		}
		responseChannel <- response
	}
}

func wsHandler(w http.ResponseWriter, r *http.Request, c <-chan AllCartsData, scenarioManager *ScenarioManager) {

	// Upgrade the HTTP connection to a WebSocket connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("Error upgrading:", err)
		return
	}
	defer conn.Close()

	// Create a channel for scenario responses
	scenarioResponseChannel := make(chan ScenarioMessage, 10)

	// Start a goroutine to handle incoming messages from the client
	go func() {
		// will this fix the panic: send on closed channel?
		for {
			// Read raw message to determine type
			_, msgBytes, err := conn.ReadMessage()
			if err != nil {
				fmt.Println("Error reading from WebSocket:", err)
				return
			}

			// Try to parse as different message types
			var rawMsg map[string]interface{}
			if err := json.Unmarshal(msgBytes, &rawMsg); err != nil {
				fmt.Println("Error parsing message:", err)
				continue
			}

			// Check if it's a test message
			if msgType, ok := rawMsg["type"].(string); ok {
				// Check if it's a scenario message
				handleScenarioMessage(msgType, rawMsg, scenarioManager, scenarioResponseChannel)
				continue
			}

			// Otherwise, treat as control message
			var msg ControlMessage
			if err := json.Unmarshal(msgBytes, &msg); err != nil {
				fmt.Println("Error parsing control message:", err)
				continue
			}

			// Process the control message
			switch msg.Command {
			case "setGoal":
				if msg.Controller >= 0 && msg.Controller < len(scenarioManager.goalChannels) {
					fmt.Printf("Frontend: Setting goal for cart %d to %f\n", msg.Controller+1, msg.Position)
					scenarioManager.goalChannels[msg.Controller] <- msg.Position
				}
			case "emergencyStop":
				if msg.Controller >= 0 && msg.Controller < len(scenarioManager.emergencyStops) {
					fmt.Printf("Frontend: Emergency stop for cart %d\n", msg.Controller+1)
					scenarioManager.emergencyStops[msg.Controller] <- true
				}
			case "randomGoals":
				fmt.Printf("Frontend: %s random goal generation\n", map[bool]string{true: "Starting", false: "Stopping"}[msg.Enabled])
				scenarioManager.randomControlChannel <- msg
			}
		}
	}()

	// Handle both cart data and scenario responses
	for {
		select {
		case value, ok := <-c:
			if !ok {
				return
			}
			err := conn.WriteJSON(value)
			if err != nil {
				fmt.Println("Error writing cart data to WebSocket:", err)
				return
			}
		case scenarioResponse, ok := <-scenarioResponseChannel:
			if !ok {
				return
			}
			err := conn.WriteJSON(scenarioResponse)
			if err != nil {
				fmt.Println("Error writing scenario response to WebSocket:", err)
				return
			}
		}
	}
}

// collectCartData collects the current data from a controller
func collectCartData(controller *Controller) SocketData {
	// Calculate trajectory phase transition timestamps and phase labels
	var trajectoryTransitions []string
	var trajectoryPhases []string

	// Map phase indices to phase names
	phaseNames := []string{
		"Increasing deceleration", // 0
		"Constant deceleration",   // 1
		"Decreasing deceleration", // 2
		"Increasing acceleration", // 3
		"Constant acceleration",   // 4
		"Decreasing acceleration", // 5
		"Constant velocity",       // 6
		"Increasing deceleration", // 7
		"Constant deceleration",   // 8
		"Decreasing deceleration", // 9
		"Final state",             // 10
	}

	if controller.CurrentTrajectory != nil {
		// Add t0 (trajectory start time) as the first transition
		trajectoryTransitions = append(trajectoryTransitions, controller.CurrentTrajectory.t0.UTC().Format(time.RFC3339Nano))
		trajectoryPhases = append(trajectoryPhases, "Start")

		// Add phase transition times and corresponding phase names
		for i, state := range controller.CurrentTrajectory.state {
			if state.t > 0 { // Only include actual phase transitions
				transitionTime := controller.CurrentTrajectory.t0.Add(time.Duration(state.t * float64(time.Second)))
				trajectoryTransitions = append(trajectoryTransitions, transitionTime.UTC().Format(time.RFC3339Nano))

				// Determine phase name based on trajectory type and index
				if controller.CurrentTrajectory.isStopping {
					// For stopping trajectories, phases start from index 0
					if i < len(phaseNames) {
						trajectoryPhases = append(trajectoryPhases, phaseNames[i])
					} else {
						trajectoryPhases = append(trajectoryPhases, "Unknown")
					}
				} else {
					// For point-to-point trajectories, phases start from index 3
					phaseIndex := i + 3
					if phaseIndex < len(phaseNames) {
						trajectoryPhases = append(trajectoryPhases, phaseNames[phaseIndex])
					} else {
						trajectoryPhases = append(trajectoryPhases, "Unknown")
					}
				}
			}
		}
	}

	// Handle potential nil trajectory for chart data
	var chartPosition, chartVelocity, chartAcceleration, chartJerk, goal float64
	if controller.CurrentTrajectory != nil {
		chartPosition = controller.CurrentTrajectory.GetCurrentPosition()
		chartVelocity = controller.CurrentTrajectory.GetCurrentVelocity()
		chartAcceleration = controller.CurrentTrajectory.GetCurrentAcceleration()
		chartJerk = controller.CurrentTrajectory.GetCurrentJerk()
		goal = controller.CurrentTrajectory.end
	}

	return SocketData{
		Id: controller.Cart.Id,

		// Chart data (planned trajectory values from MovementPlanner)
		ChartPosition:     chartPosition,
		ChartVelocity:     chartVelocity,
		ChartAcceleration: chartAcceleration,
		ChartJerk:         chartJerk,

		// Real-time data (actual cart physics values)
		Position: controller.Cart.Position,

		LeftBorder:  controller.LeftBorderTrajectory.GetCurrentPosition(),
		RightBorder: controller.RightBorderTrajectory.GetCurrentPosition(),
		Goal:        goal,
		Setpoint:    controller.PositionPID.Setpoint,
		State:       controller.State.String(),

		// Trajectory phase transitions (timestamps when trajectory phases change)
		TrajectoryTransitions: trajectoryTransitions,

		// Trajectory phase information (phase labels corresponding to transitions)
		TrajectoryPhases: trajectoryPhases,

		// Performance metrics
		Metrics: controller.Metrics.GetDetailedMetrics(),
	}
}

func startWebsocketServer(scenarioManager *ScenarioManager) {

	// Use the provided scenario manager

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

			// Use scenario manager's current controllers (which may be fewer than the original)
			if scenarioManager.controllers != nil {
				for _, controller := range scenarioManager.controllers {
					if controller != nil {
						data := collectCartData(controller)
						cartsData = append(cartsData, data)
					}
				}
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

	// Register HTTP handlers
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		wsHandler(w, r, dataChannel, scenarioManager)
	})
	// http.HandleFunc("/api/historical-data", historicalDataHandler)

	fmt.Println("WebSocket server started on :8080")
	// fmt.Println("Historical data API available at :8080/api/historical-data")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}
