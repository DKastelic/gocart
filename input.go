package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
)

func input_loop(controllerGoalChannels []chan<- float64, exit_channel chan struct{}) {
	in := bufio.NewReader(os.Stdin)

	stop_generation_channels := make([]chan struct{}, len(controllerGoalChannels))

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
			if err != nil || controllerIndexInt < 1 || controllerIndexInt >= len(controllerGoalChannels)+1 {
				fmt.Println("Invalid controller index:", controllerIndex)
				continue
			}
			goalPositionFloat, err := strconv.ParseFloat(goalPosition, 64)
			if err != nil {
				fmt.Println("Invalid goal position:", goalPosition)
				continue
			}
			controllerGoalChannels[controllerIndexInt-1] <- goalPositionFloat

		case "random":
			if len(words) < 2 {
				fmt.Println("Usage: random [on|off]")
				continue
			}
			generateRandomGoals := words[1] == "on"

			if generateRandomGoals {
				fmt.Println("Starting automatic goal generation...")
				for i, ch := range controllerGoalChannels {
					stop_generation_channels[i] = make(chan struct{})
					go func(index int, channel chan<- float64, stop chan struct{}) {
						for {
							select {
							case <-stop:
								return
							default:
								goal := rand.Float64()*1200 + 200 // Random goal between 200 and 1400
								channel <- goal
								fmt.Printf("Generated goal for controller %d: %f\n", index, goal)
							}
						}
					}(i, ch, stop_generation_channels[i])
				}
			} else {
				fmt.Println("Stopping automatic goal generation...")
				for _, stop := range stop_generation_channels {
					stop <- struct{}{}
				}
			}

		default:
			fmt.Println("Unknown command:", input)
		}
	}
}
