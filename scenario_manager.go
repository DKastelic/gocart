package main

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// CoordinationScenario represents a specific coordination test scenario
type CoordinationScenario struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`   // "idle", "running", "completed", "failed"
	Category    string `json:"category"` // "single", "two_agent", "three_agent"
}

// NetworkConfig holds network simulation parameters for scenarios
type NetworkConfig struct {
	MinDelay        time.Duration
	MaxDelay        time.Duration
	LossProbability float64
}

// ScenarioManager manages and executes coordination scenarios
type ScenarioManager struct {
	originalControllers    []*Controller    // Store original 4-cart setup
	originalGoalChannels   []chan<- float64 // Store original goal channels
	originalEmergencyStops []chan<- bool    // Store original emergency stops
	originalCarts          []Cart           // Store original carts

	controllers       []*Controller
	carts             []Cart
	goalChannels      []chan<- float64
	emergencyStops    []chan<- bool
	scenarios         []CoordinationScenario
	currentStatus     map[string]string
	activeCartCount   int
	originalCartCount int

	// Network simulation
	currentNetworkConfig NetworkConfig
	networkSimulators    []*NetworkDelaySimulator

	// Goal manager integration
	goalManager                  *GoalManager
	randomControlChannel         chan ControlMessage
	controllerCompletionChannels []<-chan bool

	// Control channels for scenario management
	exitChannel        chan struct{}
	physicsExitChannel chan struct{}
	controllerStops    []chan struct{}
	isRunning          bool

	mu sync.RWMutex
}

// NewScenarioManager creates a new scenario manager
func NewScenarioManager(controllers []*Controller, goalChannels []chan<- float64, emergencyStops []chan<- bool, carts []Cart, randomControlChannel chan ControlMessage, controllerCompletionChannels []<-chan bool) *ScenarioManager {
	scenarios := []CoordinationScenario{
		// Default scenario (original 4-cart setup)
		{Name: "Privzeti scenarij", Description: "Default 4-cart configuration for general testing", Status: "idle", Category: "multi_agent"},

		// Single Agent Scenarios
		{Name: "Preprost premik", Description: "Agent receives a goal within its borders and moves to it", Status: "idle", Category: "single"},
		{Name: "Zavrnjen cilj", Description: "Agent receives a goal outside its borders (no neighbor present) and rejects it", Status: "idle", Category: "single"},
		{Name: "Ustavitev gibanja", Description: "While moving towards a valid goal, the agent receives a stop request and halts as quickly as possible", Status: "idle", Category: "single"},
		{Name: "Sprememba cilja med gibanjem", Description: "Agent starts moving towards one goal, then receives a new goal mid movement", Status: "idle", Category: "single"},

		// Two Agent Scenarios
		{Name: "Premik meje", Description: "Agent requests a goal that requires the neighbor to shift its border but not vacate", Status: "idle", Category: "two_agent"},
		{Name: "Umik soseda", Description: "Agent requests a goal requiring the neighbor to both move the border and relocate itself", Status: "idle", Category: "two_agent"},
		{Name: "Odložena zahteva", Description: "Agent requests a goal while the neighbor is busy and must wait", Status: "idle", Category: "two_agent"},
		{Name: "Navzkrižni cilji", Description: "Both agents simultaneously request goals requiring the other to move", Status: "idle", Category: "two_agent"},
		{Name: "Prekinjen cilj", Description: "While moving, an agent receives a neighbor's request to vacate space", Status: "idle", Category: "two_agent"},
		{Name: "Prekinjeno umikanje", Description: "Neighbor changes to a new goal mid-avoidance, requiring further coordination", Status: "idle", Category: "two_agent"},

		// Three Agent Scenarios
		{Name: "Verižne zahteve", Description: "Agent requests a goal in the third agent's territory, requiring multi-hop negotiation", Status: "idle", Category: "three_agent"},
		{Name: "Nedosegljiv cilj", Description: "Agent requests a goal beyond the collective reachable space, and the farthest agent rejects it", Status: "idle", Category: "three_agent"},
		{Name: "Nezanesljivo omrežje", Description: "Chained requests with packet loss - system must converge without collision", Status: "idle", Category: "three_agent"},
		{Name: "Počasno omrežje", Description: "Chained requests with high latency - agents must handle delayed responses safely", Status: "idle", Category: "three_agent"},
		{Name: "Navzkrižni cilji z vmesnim agentom", Description: "Two agents simultaneously initiate requests requiring the middle agent to cooperate", Status: "idle", Category: "three_agent"},
	}

	var activeCartCount, originalCartCount int
	var controllerStops []chan struct{}

	if controllers != nil {
		activeCartCount = len(controllers)
		originalCartCount = len(controllers)
		controllerStops = make([]chan struct{}, len(controllers))
	} else {
		activeCartCount = 0
		originalCartCount = len(carts) // Use template cart count
		controllerStops = make([]chan struct{}, 0)
	}

	sm := &ScenarioManager{
		// Store original setup (may be nil initially)
		originalControllers:    controllers,
		originalGoalChannels:   goalChannels,
		originalEmergencyStops: emergencyStops,
		originalCarts:          make([]Cart, len(carts)),

		// Current active setup (starts as copy of original, may be empty initially)
		controllers:        controllers,
		carts:              carts,
		goalChannels:       goalChannels,
		emergencyStops:     emergencyStops,
		scenarios:          scenarios,
		currentStatus:      make(map[string]string),
		activeCartCount:    activeCartCount,
		originalCartCount:  originalCartCount,
		exitChannel:        make(chan struct{}),
		physicsExitChannel: make(chan struct{}),
		controllerStops:    controllerStops,
		isRunning:          false,

		// Network simulation - default to low latency, no packet loss
		currentNetworkConfig: NetworkConfig{
			MinDelay:        10 * time.Millisecond,
			MaxDelay:        20 * time.Millisecond,
			LossProbability: 0.0,
		},
		networkSimulators: make([]*NetworkDelaySimulator, 0),

		// Goal manager integration
		randomControlChannel:         randomControlChannel,
		controllerCompletionChannels: controllerCompletionChannels,
	}

	// Copy original carts
	copy(sm.originalCarts, carts)

	// Initialize goal manager only if we have initial controllers
	if controllers != nil && goalChannels != nil && controllerCompletionChannels != nil {
		sm.goalManager = NewGoalManager(sm.goalChannels, sm.controllerCompletionChannels, sm.randomControlChannel)
		sm.goalManager.Start()
	}

	return sm
}

// =====================================================
// GOAL MANAGER AND NETWORK MANAGEMENT
// =====================================================

// updateGoalManager recreates and restarts the goal manager for the current cart configuration
func (sm *ScenarioManager) updateGoalManager() {
	log.Printf("[SCENARIO] Updating goal manager for %d cart configuration", len(sm.controllers))

	// Verify we have controllers and channels
	if len(sm.controllers) == 0 {
		log.Println("[SCENARIO] No controllers available, skipping goal manager setup")
		return
	}

	if len(sm.goalChannels) != len(sm.controllers) {
		log.Printf("[SCENARIO] WARNING: Goal channels (%d) don't match controllers (%d)",
			len(sm.goalChannels), len(sm.controllers))
		return
	}

	// Recreate completion channels for current controllers
	sm.controllerCompletionChannels = make([]<-chan bool, len(sm.controllers))
	for i := range sm.controllers {
		if sm.controllers[i] == nil {
			log.Printf("[SCENARIO] WARNING: Controller %d is nil", i)
			continue
		}
		sm.controllerCompletionChannels[i] = sm.controllers[i].GoalCompletionReport
		log.Printf("[SCENARIO] Connected goal manager to controller %d", i+1)
	}

	// Update existing goal manager or create new one if needed
	if sm.goalManager != nil {
		log.Println("[SCENARIO] Updating existing goal manager channels")
		sm.goalManager.updateChannels(sm.goalChannels, sm.controllerCompletionChannels)
	} else {
		// Create new goal manager with current configuration
		log.Printf("[SCENARIO] Creating new goal manager with %d goal channels and %d completion channels",
			len(sm.goalChannels), len(sm.controllerCompletionChannels))
		sm.goalManager = NewGoalManager(sm.goalChannels, sm.controllerCompletionChannels, sm.randomControlChannel)
		sm.goalManager.Start()
	}

	log.Println("[SCENARIO] Goal manager successfully updated")
}

// setNetworkConfig updates the network configuration for scenarios
func (sm *ScenarioManager) setNetworkConfig(config NetworkConfig) {
	sm.currentNetworkConfig = config
	log.Printf("[SCENARIO] Network config updated: minDelay=%v, maxDelay=%v, loss=%.3f",
		config.MinDelay, config.MaxDelay, config.LossProbability)
}

// connectControllersWithConfig connects controllers using the current network configuration
func (sm *ScenarioManager) connectControllersWithConfig(leftController, rightController *Controller) *NetworkDelaySimulator {
	// Create network simulator with current config
	networkSim := NewNetworkDelaySimulator(
		sm.currentNetworkConfig.MinDelay,
		sm.currentNetworkConfig.MaxDelay,
		sm.currentNetworkConfig.LossProbability,
	)

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

	return networkSim
}

// initializeScenarioMetrics resets metrics for all controllers at the start of a scenario
func (sm *ScenarioManager) initializeScenarioMetrics() {
	for _, controller := range sm.controllers {
		if controller != nil && controller.Metrics != nil {
			controller.Metrics.StartScenario()
		}
	}
}

// GetGoalManager returns the current goal manager instance
func (sm *ScenarioManager) GetGoalManager() *GoalManager {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.goalManager
}

// GetScenarios returns the list of available scenarios
func (sm *ScenarioManager) GetScenarios() []CoordinationScenario {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make([]CoordinationScenario, len(sm.scenarios))
	copy(result, sm.scenarios)

	// Update status from currentStatus map
	for i := range result {
		if status, exists := sm.currentStatus[result[i].Name]; exists {
			result[i].Status = status
		}
	}

	return result
}

// RunScenario executes a specific coordination scenario
func (sm *ScenarioManager) RunScenario(scenarioName string) error {
	sm.mu.Lock()
	sm.currentStatus[scenarioName] = "running"
	sm.mu.Unlock()

	log.Printf("[SCENARIO] Starting scenario: %s", scenarioName)

	// Initialize metrics for all controllers in the scenario
	sm.initializeScenarioMetrics()

	var err error
	switch scenarioName {
	// Default scenario
	case "Privzeti scenarij":
		err = sm.runDefault()

	// Single Agent Scenarios
	case "Preprost premik":
		err = sm.runSimpleMove()
	case "Zavrnjen cilj":
		err = sm.runSimpleReject()
	case "Ustavitev gibanja":
		err = sm.runStopMovement()
	case "Sprememba cilja med gibanjem":
		err = sm.runChangeGoalMidMovement()

	// Two Agent Scenarios
	case "Premik meje":
		err = sm.runBorderMove()
	case "Umik soseda":
		err = sm.runNeighborMove()
	case "Odložena zahteva":
		err = sm.runPostponedRequest()
	case "Navzkrižni cilji":
		err = sm.runCrossedGoals()
	case "Prekinjen cilj":
		err = sm.runOverriddenGoal()
	case "Prekinjeno umikanje":
		err = sm.runChangeOfPlans()

	// Three Agent Scenarios
	case "Verižne zahteve":
		err = sm.runChainedRequests()
	case "Nedosegljiv cilj":
		err = sm.runTooFar()
	case "Nezanesljivo omrežje":
		err = sm.runUnreliableNetwork()
	case "Počasno omrežje":
		err = sm.runSlowNetwork()
	case "Navzkrižni cilji z vmesnim agentom":
		err = sm.runConcurrentChainedRequests()

	default:
		err = fmt.Errorf("unknown scenario: %s", scenarioName)
	}

	sm.mu.Lock()
	if err != nil {
		sm.currentStatus[scenarioName] = "failed"
		log.Printf("[SCENARIO] Scenario %s failed: %v", scenarioName, err)
	} else {
		sm.currentStatus[scenarioName] = "completed"
		log.Printf("[SCENARIO] Scenario %s completed successfully", scenarioName)
	}
	sm.mu.Unlock()

	return err
}

// =====================================================
// SCENARIO CONTROL AND CART MANAGEMENT
// =====================================================

// runDefault implements the default 4-cart scenario
func (sm *ScenarioManager) runDefault() error {
	log.Println("[SCENARIO] Default: Setting up standard 4-cart configuration")

	sm.resetCartsWithCount(4)
	log.Println("[SCENARIO] Default configuration active. System ready for manual goals.")

	// Default scenario runs indefinitely until another scenario is started
	return nil
}

// =====================================================
// CART CONFIGURATION UTILITIES
// =====================================================

// resetCartsWithCount resets carts with a specific count for the scenario by creating new instances
func (sm *ScenarioManager) resetCartsWithCount(cartCount int) {
	log.Printf("[SCENARIO] Resetting to %d cart configuration with new instances", cartCount)

	// Stop all current controllers
	sm.stopAllControllers()
	time.Sleep(100 * time.Millisecond) // Give time for controllers to stop

	// Create new cart instances based on original carts
	sm.carts = make([]Cart, cartCount)
	for i := 0; i < cartCount; i++ {
		// Copy from original carts with proper spacing
		sm.carts[i] = sm.originalCarts[i]
		// Reset positions based on cart count for proper spacing
		switch cartCount {
		case 1:
			sm.carts[i].Position = 400.0 // Center of field
		case 2:
			positions := []float64{400.0, 1200.0}
			sm.carts[i].Position = positions[i]
		case 3:
			positions := []float64{300.0, 800.0, 1300.0}
			sm.carts[i].Position = positions[i]
		case 4:
			// Use original positions
			sm.carts[i].Position = sm.originalCarts[i].Position
		}
		// Reset motion state
		sm.carts[i].Velocity = 0
		sm.carts[i].Acceleration = 0
		sm.carts[i].Force = 0
	}

	// Create new controller instances
	sm.controllers = make([]*Controller, cartCount)
	sm.goalChannels = make([]chan<- float64, cartCount)
	sm.emergencyStops = make([]chan<- bool, cartCount)

	// Define territories based on cart count
	var territoryBounds [][]float64
	switch cartCount {
	case 1:
		territoryBounds = [][]float64{{100, 1500}}
	case 2:
		territoryBounds = [][]float64{{50, 800}, {800, 1550}}
	case 3:
		territoryBounds = [][]float64{{25, 533}, {533, 1066}, {1066, 1575}}
	case 4:
		territoryBounds = [][]float64{{0, 400}, {400, 800}, {800, 1200}, {1200, 1600}}
	}

	// Create new controllers with their territories
	for i := 0; i < cartCount; i++ {
		sm.controllers[i] = NewController(&sm.carts[i], territoryBounds[i][0], territoryBounds[i][1])

		// Create new goal and emergency channels
		goalCh := make(chan float64, 10)
		emergencyCh := make(chan bool, 10)

		sm.controllers[i].IncomingGoalRequest = goalCh
		sm.controllers[i].IncomingEmergencyStop = emergencyCh

		sm.goalChannels[i] = goalCh
		sm.emergencyStops[i] = emergencyCh

		// Start the controller
		go sm.controllers[i].run_controller()
		log.Printf("[SCENARIO] Created and started new controller %d with territory [%.0f, %.0f]",
			i+1, territoryBounds[i][0], territoryBounds[i][1])
	}

	// Connect controllers for coordination with current network config
	sm.networkSimulators = make([]*NetworkDelaySimulator, 0)
	for i := 0; i < len(sm.controllers)-1; i++ {
		networkSim := sm.connectControllersWithConfig(sm.controllers[i], sm.controllers[i+1])
		sm.networkSimulators = append(sm.networkSimulators, networkSim)
	}

	sm.activeCartCount = cartCount

	// Update goal manager for new cart configuration
	sm.updateGoalManager()

	log.Printf("[SCENARIO] Successfully created %d new cart instances with network config", cartCount)
}

// stopAllControllers stops all running controllers
func (sm *ScenarioManager) stopAllControllers() {
	log.Println("[SCENARIO] Stopping all controllers")
	for i, controller := range sm.controllers {
		if controller.StopController != nil {
			select {
			case controller.StopController <- struct{}{}:
				log.Printf("[SCENARIO] Sent stop signal to controller %d", i+1)
			default:
				// Channel might be closed or full, skip
			}
		}
	}
}

// =====================================================
// SINGLE AGENT SCENARIOS
// =====================================================

func (sm *ScenarioManager) runSimpleMove() error {
	log.Println("[SCENARIO] Simple Move: Agent receives a goal within its borders and moves to it")

	sm.resetCartsWithCount(1)
	time.Sleep(500 * time.Millisecond)

	// Single cart can use most of the field
	goal := 1200.0 // Move to 1200

	select {
	case sm.goalChannels[0] <- goal:
		log.Printf("[SCENARIO] Cart 1 goal sent to position %.0f", goal)
	case <-time.After(1 * time.Second):
		return fmt.Errorf("timeout sending goal to Cart 1")
	}

	time.Sleep(8 * time.Second)
	return nil
}

func (sm *ScenarioManager) runSimpleReject() error {
	log.Println("[SCENARIO] Simple Reject: Agent receives a goal outside its borders (no neighbor present)")

	sm.resetCartsWithCount(1)
	time.Sleep(500 * time.Millisecond)

	// Send a goal way outside the field bounds
	goal := 2000.0 // Way beyond any reachable space
	select {
	case sm.goalChannels[0] <- goal:
		log.Printf("[SCENARIO] Cart 1 goal sent to position %.0f (should be rejected)", goal)
	case <-time.After(1 * time.Second):
		return fmt.Errorf("timeout sending goal to Cart 1")
	}

	time.Sleep(1 * time.Second)
	return nil
}

func (sm *ScenarioManager) runStopMovement() error {
	log.Println("[SCENARIO] Stop Movement: While moving towards a valid goal, agent receives a stop request")

	sm.resetCartsWithCount(1)
	time.Sleep(500 * time.Millisecond)

	// Send single cart a goal across the field
	goal := 1200.0

	select {
	case sm.goalChannels[0] <- goal:
		log.Printf("[SCENARIO] Cart 1 goal sent to position %.0f", goal)
	case <-time.After(1 * time.Second):
		return fmt.Errorf("timeout sending goal to Cart 1")
	}

	// Wait for movement to start, then send emergency stop
	time.Sleep(2 * time.Second)

	log.Println("[SCENARIO] Triggering emergency stop while moving!")
	select {
	case sm.emergencyStops[0] <- true:
		log.Println("[SCENARIO] Emergency stop sent to Cart 1")
	case <-time.After(100 * time.Millisecond):
		return fmt.Errorf("timeout sending emergency stop to Cart 1")
	}

	time.Sleep(5 * time.Second)
	return nil
}

func (sm *ScenarioManager) runChangeGoalMidMovement() error {
	log.Println("[SCENARIO] Change Goal Mid Movement: Agent starts moving towards one goal, then receives a new goal mid movement")

	sm.resetCartsWithCount(1)
	time.Sleep(500 * time.Millisecond)

	// Send single cart initial goal (towards one end)
	goal1 := 1200.0

	select {
	case sm.goalChannels[0] <- goal1:
		log.Printf("[SCENARIO] Cart 1 initial goal sent to position %.0f", goal1)
	case <-time.After(1 * time.Second):
		return fmt.Errorf("timeout sending initial goal to Cart 1")
	}

	// Wait for movement to start, then send opposite direction goal
	time.Sleep(2000 * time.Millisecond)

	goal2 := 400.0
	select {
	case sm.goalChannels[0] <- goal2:
		log.Printf("[SCENARIO] Cart 1 opposite goal sent to position %.0f", goal2)
	case <-time.After(1 * time.Second):
		return fmt.Errorf("timeout sending opposite goal to Cart 1")
	}

	time.Sleep(8 * time.Second)
	return nil
}

// =====================================================
// TWO AGENT SCENARIOS
// =====================================================

func (sm *ScenarioManager) runBorderMove() error {
	log.Println("[SCENARIO] Border Move: Agent requests a goal that requires neighbor to shift border but not vacate")

	sm.resetCartsWithCount(2)
	time.Sleep(500 * time.Millisecond)

	// Cart 1 wants to move slightly into Cart 2's territory
	// With 2 carts: Cart 1 (0-800), Cart 2 (800-1600)
	goal := 850.0 // Just into Cart 2's territory
	select {
	case sm.goalChannels[0] <- goal:
		log.Printf("[SCENARIO] Cart 1 goal sent to position %.0f (requires border shift from Cart 2)", goal)
	case <-time.After(1 * time.Second):
		return fmt.Errorf("timeout sending goal to Cart 1")
	}

	time.Sleep(10 * time.Second)
	return nil
}

func (sm *ScenarioManager) runNeighborMove() error {
	log.Println("[SCENARIO] Neighbor Move: Agent requests a goal requiring neighbor to move border and relocate itself")

	sm.resetCartsWithCount(2)
	time.Sleep(500 * time.Millisecond)

	// Cart 1 wants to move deep into Cart 2's territory, requiring Cart 2 to relocate
	goal := 1400.0 // Deep into Cart 2's territory (800-1500)
	select {
	case sm.goalChannels[0] <- goal:
		log.Printf("[SCENARIO] Cart 1 goal sent to position %.0f (requires Cart 2 to relocate)", goal)
	case <-time.After(1 * time.Second):
		return fmt.Errorf("timeout sending goal to Cart 1")
	}

	time.Sleep(12 * time.Second)
	return nil
}

func (sm *ScenarioManager) runPostponedRequest() error {
	log.Println("[SCENARIO] Postponed Request: Agent requests a goal while neighbor is busy, must wait")

	sm.resetCartsWithCount(2)
	time.Sleep(500 * time.Millisecond)

	// First make Cart 2 busy with its own goal
	goal2 := 1200.0
	select {
	case sm.goalChannels[1] <- goal2:
		log.Printf("[SCENARIO] Cart 2 goal sent to position %.0f (making it busy)", goal2)
	case <-time.After(1 * time.Second):
		return fmt.Errorf("timeout sending goal to Cart 2")
	}

	// Wait a moment, then send Cart 1 a goal that requires Cart 2 to move out of the way
	time.Sleep(1 * time.Second)

	goal1 := 1200.0 // Requires Cart 2's cooperation while it's busy
	select {
	case sm.goalChannels[0] <- goal1:
		log.Printf("[SCENARIO] Cart 1 goal sent to position %.0f (should be postponed)", goal1)
	case <-time.After(1 * time.Second):
		return fmt.Errorf("timeout sending goal to Cart 1")
	}

	time.Sleep(15 * time.Second)
	return nil
}

func (sm *ScenarioManager) runCrossedGoals() error {
	log.Println("[SCENARIO] Crossed Goals: Both agents simultaneously request goals requiring the other to move")

	sm.resetCartsWithCount(2)
	time.Sleep(500 * time.Millisecond)

	// Send crossed goals simultaneously
	// Cart 1 (starts at 400) wants Cart 2's territory, Cart 2 (starts at 1200) wants Cart 1's territory
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		select {
		case sm.goalChannels[0] <- 1100.0: // Cart 1 to Cart 2's territory
			log.Println("[SCENARIO] Cart 1 goal sent to position 1100 (crosses into Cart 2's territory)")
		case <-time.After(1 * time.Second):
			log.Println("[SCENARIO] Timeout sending goal to Cart 1")
		}
	}()

	go func() {
		defer wg.Done()
		select {
		case sm.goalChannels[1] <- 500.0: // Cart 2 to Cart 1's territory
			log.Println("[SCENARIO] Cart 2 goal sent to position 500 (crosses into Cart 1's territory)")
		case <-time.After(1 * time.Second):
			log.Println("[SCENARIO] Timeout sending goal to Cart 2")
		}
	}()

	wg.Wait()
	log.Println("[SCENARIO] Crossed goals sent simultaneously. Watch coordination!")

	time.Sleep(15 * time.Second)
	return nil
}

func (sm *ScenarioManager) runOverriddenGoal() error {
	log.Println("[SCENARIO] Overridden Goal: While moving, agent receives neighbor's request to vacate space")

	sm.resetCartsWithCount(2)
	time.Sleep(500 * time.Millisecond)

	// Start Cart 1 moving towards a goal
	goal1 := 700.0
	select {
	case sm.goalChannels[0] <- goal1:
		log.Printf("[SCENARIO] Cart 1 goal sent to position %.0f", goal1)
	case <-time.After(1 * time.Second):
		return fmt.Errorf("timeout sending goal to Cart 1")
	}

	// Wait for movement to start, then send Cart 2 a goal that requires Cart 1's space
	time.Sleep(1000 * time.Millisecond)

	goal2 := 250.0 // Cart 2 wants Cart 1's current area
	select {
	case sm.goalChannels[1] <- goal2:
		log.Printf("[SCENARIO] Cart 2 goal sent to position %.0f (should override Cart 1's movement)", goal2)
	case <-time.After(1 * time.Second):
		return fmt.Errorf("timeout sending goal to Cart 2")
	}

	time.Sleep(12 * time.Second)
	return nil
}

func (sm *ScenarioManager) runChangeOfPlans() error {
	log.Println("[SCENARIO] Change of Plans: Neighbor changes to new goal mid-avoidance, requiring further coordination")

	sm.resetCartsWithCount(2)
	time.Sleep(500 * time.Millisecond)

	// Start by sending cart 1 deep into cart 2's territory
	goal1 := 1400.0
	select {
	case sm.goalChannels[0] <- goal1:
		log.Printf("[SCENARIO] Cart 1 goal sent to position %.0f (crosses into Cart 2's territory)", goal1)
	case <-time.After(1 * time.Second):
		return fmt.Errorf("timeout sending goal to Cart 1")
	}

	// Wait for coordination to start, then change Cart 2's plan
	time.Sleep(1 * time.Second)

	newGoal2 := 150.0 // Cart 2 changes to different goal
	select {
	case sm.goalChannels[1] <- newGoal2:
		log.Printf("[SCENARIO] Cart 2 changed plans to position %.0f mid-coordination", newGoal2)
	case <-time.After(1 * time.Second):
		return fmt.Errorf("timeout sending new goal to Cart 2")
	}

	time.Sleep(12 * time.Second)
	return nil
}

// =====================================================
// THREE AGENT SCENARIOS
// =====================================================

func (sm *ScenarioManager) runChainedRequests() error {
	log.Println("[SCENARIO] Chained Requests: Agent requests goal in third agent's territory, requiring multi-hop negotiation")

	sm.resetCartsWithCount(3)
	time.Sleep(500 * time.Millisecond)

	// Cart 1 wants to reach Cart 3's territory, requiring coordination through Cart 2
	// With 3 carts: Cart 1 (0-533), Cart 2 (533-1067), Cart 3 (1067-1600)
	goal := 1300.0 // Deep into Cart 3's territory
	select {
	case sm.goalChannels[0] <- goal:
		log.Printf("[SCENARIO] Cart 1 goal sent to position %.0f (requires chained negotiation through Cart 2)", goal)
	case <-time.After(1 * time.Second):
		return fmt.Errorf("timeout sending goal to Cart 1")
	}

	time.Sleep(15 * time.Second)
	return nil
}

func (sm *ScenarioManager) runTooFar() error {
	log.Println("[SCENARIO] Too Far: Agent requests goal beyond collective reachable space, farthest agent rejects")

	sm.resetCartsWithCount(3)
	time.Sleep(500 * time.Millisecond)

	// Cart 1 wants to reach way beyond Cart 3's territory
	goal := 1800.0 // Way beyond any reachable space
	select {
	case sm.goalChannels[0] <- goal:
		log.Printf("[SCENARIO] Cart 1 goal sent to position %.0f (should be rejected as too far)", goal)
	case <-time.After(1 * time.Second):
		return fmt.Errorf("timeout sending goal to Cart 1")
	}

	time.Sleep(10 * time.Second)
	return nil
}

func (sm *ScenarioManager) runUnreliableNetwork() error {
	log.Println("[SCENARIO] Unreliable Network: Chained requests with packet loss, system must converge without collision")

	// Set network config with packet loss for this scenario
	sm.setNetworkConfig(NetworkConfig{
		MinDelay:        10 * time.Millisecond,
		MaxDelay:        20 * time.Millisecond,
		LossProbability: 0.15, // 15% packet loss
	})

	sm.resetCartsWithCount(3)
	time.Sleep(500 * time.Millisecond)

	// Enable packet loss simulation for this scenario
	log.Println("[SCENARIO] Simulating packet loss during chained negotiation")

	// Cart 1 wants to reach Cart 3's territory with unreliable network
	goal := 1300.0
	select {
	case sm.goalChannels[0] <- goal:
		log.Printf("[SCENARIO] Cart 1 goal sent to position %.0f with network unreliability", goal)
	case <-time.After(1 * time.Second):
		return fmt.Errorf("timeout sending goal to Cart 1")
	}

	time.Sleep(20 * time.Second) // Longer time due to retries

	// Reset to default network config
	sm.setNetworkConfig(NetworkConfig{
		MinDelay:        10 * time.Millisecond,
		MaxDelay:        15 * time.Millisecond,
		LossProbability: 0.0,
	})

	return nil
}

func (sm *ScenarioManager) runSlowNetwork() error {
	log.Println("[SCENARIO] Slow Network: Chained requests with high latency, agents must handle delayed responses")

	// Set network config with high latency for this scenario
	sm.setNetworkConfig(NetworkConfig{
		MinDelay:        100 * time.Millisecond,
		MaxDelay:        300 * time.Millisecond,
		LossProbability: 0.0, // No packet loss, just high latency
	})

	sm.resetCartsWithCount(3)
	time.Sleep(500 * time.Millisecond)

	log.Println("[SCENARIO] Simulating high network latency during chained negotiation")

	// Cart 1 wants to reach Cart 3's territory with slow network
	goal := 1300.0
	select {
	case sm.goalChannels[0] <- goal:
		log.Printf("[SCENARIO] Cart 1 goal sent to position %.0f with high latency", goal)
	case <-time.After(1 * time.Second):
		return fmt.Errorf("timeout sending goal to Cart 1")
	}

	time.Sleep(25 * time.Second) // Much longer time due to latency

	// Reset to default network config
	sm.setNetworkConfig(NetworkConfig{
		MinDelay:        10 * time.Millisecond,
		MaxDelay:        15 * time.Millisecond,
		LossProbability: 0.0,
	})

	return nil
}

func (sm *ScenarioManager) runConcurrentChainedRequests() error {
	log.Println("[SCENARIO] Concurrent Chained Requests: Two agents simultaneously initiate requests requiring middle agent cooperation")

	sm.resetCartsWithCount(3)
	time.Sleep(500 * time.Millisecond)

	// Cart 1 and Cart 3 simultaneously request goals that require Cart 2's cooperation
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		select {
		case sm.goalChannels[0] <- 1400.0: // Cart 1 to Cart 3's territory
			log.Println("[SCENARIO] Cart 1 goal sent to position 1400 (requires Cart 2's cooperation)")
		case <-time.After(1 * time.Second):
			log.Println("[SCENARIO] Timeout sending goal to Cart 1")
		}
	}()

	go func() {
		defer wg.Done()
		select {
		case sm.goalChannels[2] <- 300.0: // Cart 3 to Cart 1's territory
			log.Println("[SCENARIO] Cart 3 goal sent to position 300 (requires Cart 2's cooperation)")
		case <-time.After(1 * time.Second):
			log.Println("[SCENARIO] Timeout sending goal to Cart 3")
		}
	}()

	wg.Wait()
	log.Println("[SCENARIO] Concurrent chained requests sent. Cart 2 must prioritize and resolve!")

	time.Sleep(20 * time.Second)
	return nil
}
