package main

import (
	"fmt"
	"time"
)

// FPS is the frames per second for the physics loop
const PHYSICS_FPS = 1000

func physics_loop(carts []Cart, exit_channel chan struct{}) {

	ticker := time.NewTicker(time.Second / PHYSICS_FPS)
	defer ticker.Stop()

	previousTime := time.Now()

	for t := range ticker.C {

		// Calculate delta time
		deltaTime := t.Sub(previousTime).Seconds()
		previousTime = t

		// Update the physics of each cart
		for i := range carts {
			cart := &carts[i]

			// Update using Newton's method (Euler's method)
			// Position derivative is velocity
			cart.Position = rk4_step(func(t, pos float64) float64 {
				return cart.Velocity
			}, float64(t.Unix()), cart.Position, deltaTime)

			// Velocity derivative is acceleration
			cart.Velocity = rk4_step(func(t, vel float64) float64 {
				return cart.Acceleration
			}, float64(t.Unix()), cart.Velocity, deltaTime)

			// Acceleration is force divided by mass
			cart.Acceleration = cart.Force / cart.Mass
		}

		// Check for collisions
		for i := 0; i < len(carts); i++ {
			for j := i + 1; j < len(carts); j++ {
				cartA := &carts[i]
				cartB := &carts[j]

				// Check for collision
				if cartA.Position+cartA.Width/2 > cartB.Position-cartB.Width/2 && cartA.Position-cartA.Width/2 < cartB.Position+cartB.Width/2 {
					// Handle collision (assumes perfectly elastic collision between carts of equal mass)
					// Swap velocities
					// This is a simple example; in a real-world scenario, you would need to consider the masses and velocities of both carts
					cartA.Velocity, cartB.Velocity = cartB.Velocity, cartA.Velocity

					fmt.Printf("Collision detected between cart %d and cart %d\n", i+1, j+1)

					// End the simulation if a collision occurs
					exit_channel <- struct{}{}
				}
			}
		}
	}
}
