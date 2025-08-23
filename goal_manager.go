package main

import (
	"fmt"
	"math/rand"
	"time"
)

// GoalManagerConfig holds configuration for the goal manager
type GoalManagerConfig struct {
	OverrideChance     float64       // Probability (0-1) of overriding current goal with new one
	MinGoalInterval    time.Duration // Minimum time between goals
	MaxGoalInterval    time.Duration // Maximum time between goals
	GoalRangeMin       float64       // Minimum goal value
	GoalRangeMax       float64       // Maximum goal value
	UsePerCartRanges   bool          // Whether to use per-cart goal ranges
	CooldownAfterFail  time.Duration // Cooldown period after goal failure/abandonment
	MinGoalPersistence time.Duration // Minimum time a goal should persist before being replaced
	StaggerDelay       time.Duration // Delay between starting goal generation for different controllers
}

// DefaultGoalManagerConfig returns default configuration
func DefaultGoalManagerConfig() GoalManagerConfig {
	return GoalManagerConfig{
		GoalRangeMin:       200,
		GoalRangeMax:       1400,
		MinGoalInterval:    2 * time.Second,        // Minimum 2 seconds between goals
		MaxGoalInterval:    5 * time.Second,        // Maximum 5 seconds between goals
		UsePerCartRanges:   true,                   // Use existing per-cart ranges
		CooldownAfterFail:  3 * time.Second,        // 3 seconds cooldown after goal failure
		MinGoalPersistence: 4 * time.Second,        // Goals must persist for at least 4 seconds
		StaggerDelay:       500 * time.Millisecond, // 500ms delay between controllers
	}
}

// GoalManager handles random goal generation for controllers
type GoalManager struct {
	controllerGoalChannels       []chan<- float64
	controllerCompletionChannels []<-chan bool
	randomControlChannel         <-chan ControlMessage
	stopChannels                 []chan struct{}
	config                       GoalManagerConfig
	isRunning                    bool
	// Track goal timing for each controller
	lastGoalTime   []time.Time // When the last goal was sent
	lastFailTime   []time.Time // When the last goal failed/was abandoned
	controllerBusy []bool      // Whether each controller is busy with a goal
}

// NewGoalManager creates a new goal manager
func NewGoalManager(controllerGoalChannels []chan<- float64, controllerCompletionChannels []<-chan bool, randomControlChannel <-chan ControlMessage) *GoalManager {
	numControllers := len(controllerGoalChannels)
	return &GoalManager{
		controllerGoalChannels:       controllerGoalChannels,
		controllerCompletionChannels: controllerCompletionChannels,
		randomControlChannel:         randomControlChannel,
		stopChannels:                 make([]chan struct{}, numControllers),
		config:                       DefaultGoalManagerConfig(),
		isRunning:                    false,
		lastGoalTime:                 make([]time.Time, numControllers),
		lastFailTime:                 make([]time.Time, numControllers),
		controllerBusy:               make([]bool, numControllers),
	}
}

// SetConfig updates the goal manager configuration
func (gm *GoalManager) SetConfig(config GoalManagerConfig) {
	gm.config = config
}

// SetReactiveConfig sets configuration optimized for reducing goal conflicts
func (gm *GoalManager) SetReactiveConfig() {
	gm.config = GoalManagerConfig{
		GoalRangeMin:       200,
		GoalRangeMax:       1400,
		MinGoalInterval:    3 * time.Second, // Longer intervals
		MaxGoalInterval:    8 * time.Second, // Much longer max interval
		UsePerCartRanges:   true,
		CooldownAfterFail:  5 * time.Second, // Longer cooldown after failures
		MinGoalPersistence: 6 * time.Second, // Goals must persist longer
		StaggerDelay:       1 * time.Second, // More stagger between controllers
	}
	fmt.Println("Goal manager configured for reactive behavior (reduced conflicts)")
}

// SetAggressiveConfig sets configuration for more dynamic/frequent goal changes
func (gm *GoalManager) SetAggressiveConfig() {
	gm.config = GoalManagerConfig{
		GoalRangeMin:       200,
		GoalRangeMax:       1400,
		MinGoalInterval:    1 * time.Second, // Shorter intervals
		MaxGoalInterval:    3 * time.Second, // Shorter max interval
		UsePerCartRanges:   true,
		CooldownAfterFail:  2 * time.Second,        // Shorter cooldown
		MinGoalPersistence: 2 * time.Second,        // Goals can change more quickly
		StaggerDelay:       200 * time.Millisecond, // Less stagger
	}
	fmt.Println("Goal manager configured for aggressive behavior (more dynamic)")
}

// Start begins handling random goal generation commands
func (gm *GoalManager) Start() {
	go gm.handleRandomGoalGeneration()
}

// handleRandomGoalGeneration processes random goal generation commands
func (gm *GoalManager) handleRandomGoalGeneration() {
	fmt.Println("Goal manager waiting for commands...")
	for msg := range gm.randomControlChannel {
		fmt.Printf("Goal manager received command: %s, enabled: %t\n", msg.Command, msg.Enabled)
		if msg.Command == "randomGoals" {
			if msg.Enabled {
				gm.startRandomGoals()
			} else {
				gm.stopRandomGoals()
			}
		}
	}
}

// startRandomGoals begins generating random goals for all controllers
func (gm *GoalManager) startRandomGoals() {
	if gm.isRunning {
		return // Already running
	}

	fmt.Println("Starting automatic goal generation...")
	gm.isRunning = true

	// Reset tracking arrays
	numControllers := len(gm.controllerGoalChannels)
	gm.lastGoalTime = make([]time.Time, numControllers)
	gm.lastFailTime = make([]time.Time, numControllers)
	gm.controllerBusy = make([]bool, numControllers)

	// Start goal managers for each controller with staggered delays
	for i := range gm.controllerGoalChannels {
		gm.stopChannels[i] = make(chan struct{})

		// Stagger the start of goal generation to reduce simultaneous conflicts
		go func(index int) {
			// Wait for staggered delay
			time.Sleep(time.Duration(index) * gm.config.StaggerDelay)
			gm.manageGoalsForController(index, gm.stopChannels[index])
		}(i)
	}
}

// stopRandomGoals stops generating random goals for all controllers
func (gm *GoalManager) stopRandomGoals() {
	if !gm.isRunning {
		return // Already stopped
	}

	fmt.Println("Stopping automatic goal generation...")
	gm.isRunning = false

	for _, stop := range gm.stopChannels {
		if stop != nil {
			close(stop)
		}
	}

	// Clear stop channels array
	gm.stopChannels = make([]chan struct{}, len(gm.controllerGoalChannels))
}

// updateChannels updates the goal manager's channels when the controller configuration changes
func (gm *GoalManager) updateChannels(controllerGoalChannels []chan<- float64, controllerCompletionChannels []<-chan bool) {
	fmt.Printf("Goal manager updating channels: %d goal channels, %d completion channels\n",
		len(controllerGoalChannels), len(controllerCompletionChannels))

	// Stop any running goal generation
	gm.stopRandomGoals()

	// Update channels
	gm.controllerGoalChannels = controllerGoalChannels
	gm.controllerCompletionChannels = controllerCompletionChannels
	numControllers := len(controllerGoalChannels)
	gm.stopChannels = make([]chan struct{}, numControllers)

	// Resize tracking arrays
	gm.lastGoalTime = make([]time.Time, numControllers)
	gm.lastFailTime = make([]time.Time, numControllers)
	gm.controllerBusy = make([]bool, numControllers)

	fmt.Printf("Goal manager successfully updated to handle %d controllers\n", len(controllerGoalChannels))
}

// manageGoalsForController manages goals for a specific controller
func (gm *GoalManager) manageGoalsForController(index int, stop chan struct{}) {
	// Send initial goal with a small delay
	gm.randomSleep()
	if !gm.sendGoalToController(index) {
		return // Stop channel was closed
	}

	for {
		select {
		case <-stop:
			return

		case result := <-gm.controllerCompletionChannels[index]:
			gm.controllerBusy[index] = false

			if result {
				fmt.Printf("Controller %d completed goal successfully\n", index+1)
				// Successful completion - wait normal interval before next goal
				gm.waitBeforeNextGoal(index, false)
			} else {
				fmt.Printf("Controller %d abandoned goal\n", index+1)
				gm.lastFailTime[index] = time.Now()
				// Failed/abandoned goal - apply cooldown period
				gm.waitBeforeNextGoal(index, true)
			}

			// Send next goal if not stopped
			select {
			case <-stop:
				return
			default:
				if !gm.sendGoalToController(index) {
					return // Stop channel was closed
				}
			}
		}
	}
}

// sendGoalToController attempts to send a goal to a controller
func (gm *GoalManager) sendGoalToController(index int) bool {
	goal := gm.generateSmartGoalForController(index)

	select {
	case gm.controllerGoalChannels[index] <- goal:
		gm.lastGoalTime[index] = time.Now()
		gm.controllerBusy[index] = true
		fmt.Printf("Generated goal for controller %d: %.2f\n", index+1, goal)
		return true
	default:
		fmt.Printf("Controller %d: goal channel full, skipping\n", index+1)
		return true
	}
}

// waitBeforeNextGoal implements smart waiting logic based on whether the last goal failed
func (gm *GoalManager) waitBeforeNextGoal(index int, failed bool) {
	var waitTime time.Duration

	if failed {
		// Apply cooldown after failure
		waitTime = gm.config.CooldownAfterFail
		fmt.Printf("Controller %d: applying %.1fs cooldown after goal failure\n", index+1, waitTime.Seconds())
	} else {
		// Check if we need to wait for minimum persistence time
		timeSinceLastGoal := time.Since(gm.lastGoalTime[index])
		if timeSinceLastGoal < gm.config.MinGoalPersistence {
			extraWait := gm.config.MinGoalPersistence - timeSinceLastGoal
			fmt.Printf("Controller %d: waiting extra %.1fs for goal persistence\n", index+1, extraWait.Seconds())
			time.Sleep(extraWait)
		}

		// Normal random interval
		waitTime = gm.getRandomInterval()
	}

	time.Sleep(waitTime)
}

// getRandomInterval returns a random interval between min and max goal intervals
func (gm *GoalManager) getRandomInterval() time.Duration {
	minNanos := int64(gm.config.MinGoalInterval)
	maxNanos := int64(gm.config.MaxGoalInterval)
	randomNanos := minNanos + rand.Int63n(maxNanos-minNanos)
	return time.Duration(randomNanos)
}

// generateGoalForController generates a goal for a specific controller
func (gm *GoalManager) generateGoalForController(index int) float64 {
	if gm.config.UsePerCartRanges {
		// Use per-cart ranges similar to original implementation
		return float64(index*400) - 200 + rand.Float64()*800 // Random goal between index*400-200 and (index+1)*400+200
	} else {
		// Use global range
		return gm.config.GoalRangeMin + rand.Float64()*(gm.config.GoalRangeMax-gm.config.GoalRangeMin)
	}
}

func (gm *GoalManager) randomSleep() {
	interval := gm.getRandomInterval()
	time.Sleep(interval)
}

// generateSmartGoalForController generates a goal that tries to avoid immediate conflicts
func (gm *GoalManager) generateSmartGoalForController(index int) float64 {
	baseGoal := gm.generateGoalForController(index)

	// Simple conflict avoidance: if recent failures, try to pick goals further from borders
	if time.Since(gm.lastFailTime[index]) < 2*gm.config.CooldownAfterFail {
		if gm.config.UsePerCartRanges {
			// Generate goals more towards the center of the cart's range
			rangeStart := float64(index*400) - 200
			rangeEnd := float64((index+1)*400) + 200
			center := (rangeStart + rangeEnd) / 2
			// Bias towards center by 60%
			return center + (baseGoal-center)*0.4
		}
	}

	return baseGoal
}
