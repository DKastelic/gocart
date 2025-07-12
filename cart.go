package main

// the cart's phyisical properties
type Cart struct {
	Name string
	Id   int

	// The cart's position
	Position float64
	// The cart's velocity
	Velocity float64
	// The cart's acceleration
	Acceleration float64
	// The cart's mass
	Mass float64
	// The cart's force
	Force float64

	// The cart's height
	Height float64
	// The cart's width
	Width float64
}

func (cart *Cart) applyForce(force float64) {
	cart.Force = force
}
