package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func input_loop(scenarioManager *ScenarioManager, exit_channel chan struct{}, randomControlChannel chan<- ControlMessage) {
	in := bufio.NewReader(os.Stdin)

	fmt.Println("Usage: \n" +
		"goal <controller_index> <goal_position> - Set a goal for a specific controller.\n" +
		"random [on|off] - Start or stop automatic goal generation.\n" +
		"exit - Exit the program.")

	for {
		input, err := in.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading input:", err)
			continue
		}
		input = strings.TrimSpace(input)

		var words = strings.Split(input, " ")

		switch words[0] {
		case "exit":
			fmt.Println("Exiting...")
			exit_channel <- struct{}{}
			return

		case "goal":
			if len(words) < 3 {
				fmt.Println("Usage: goal <controller_index> <goal_position>")
				continue
			}
			controllerIndex := words[1]
			goalPosition := words[2]
			fmt.Printf("Setting goal for controller %s to position %s\n", controllerIndex,
				goalPosition)
			controllerIndexInt, err := strconv.Atoi(controllerIndex)
			if err != nil || controllerIndexInt < 1 || controllerIndexInt >= len(scenarioManager.goalChannels)+1 {
				fmt.Println("Invalid controller index:", controllerIndex)
				continue
			}
			goalPositionFloat, err := strconv.ParseFloat(goalPosition, 64)
			if err != nil {
				fmt.Println("Invalid goal position:", goalPosition)
				continue
			}
			scenarioManager.goalChannels[controllerIndexInt-1] <- goalPositionFloat

		case "random":
			if len(words) < 2 {
				fmt.Println("Usage: random [on|off]")
				continue
			}
			generateRandomGoals := words[1] == "on"

			// Send control message to goal manager
			controlMsg := ControlMessage{
				Command: "randomGoals",
				Enabled: generateRandomGoals,
			}
			randomControlChannel <- controlMsg

		default:
			fmt.Println("Unknown command:", input)
		}
	}
}
