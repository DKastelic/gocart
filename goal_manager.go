package main

import (
	"fmt"
	"math/rand"
	"time"
)

// GoalManager handles random goal generation for controllers
type GoalManager struct {
	controllerGoalChannels []chan<- float64
	randomControlChannel   <-chan ControlMessage
	stopChannels           []chan struct{}
	isRunning              bool
}

// NewGoalManager creates a new goal manager
func NewGoalManager(controllerGoalChannels []chan<- float64, randomControlChannel <-chan ControlMessage) *GoalManager {
	return &GoalManager{
		controllerGoalChannels: controllerGoalChannels,
		randomControlChannel:   randomControlChannel,
		stopChannels:           make([]chan struct{}, len(controllerGoalChannels)),
		isRunning:              false,
	}
}

// Start begins handling random goal generation commands
func (gm *GoalManager) Start() {
	go gm.handleRandomGoalGeneration()
}

// handleRandomGoalGeneration processes random goal generation commands
func (gm *GoalManager) handleRandomGoalGeneration() {
	for msg := range gm.randomControlChannel {
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

	for i, ch := range gm.controllerGoalChannels {
		gm.stopChannels[i] = make(chan struct{})
		go gm.generateGoalsForController(i, ch, gm.stopChannels[i])
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
}

// generateGoalsForController generates random goals for a specific controller
func (gm *GoalManager) generateGoalsForController(index int, channel chan<- float64, stop chan struct{}) {
	for {
		select {
		case <-stop:
			return
		default:
			goal := rand.Float64()*1200 + 200 // Random goal between 200 and 1400
			channel <- goal
			time.Sleep(time.Duration(rand.Intn(2000)) * time.Millisecond) // Random delay between 0 and 2 seconds
			fmt.Printf("Generated random goal for controller %d: %.2f\n", index+1, goal)
		}
	}
}
