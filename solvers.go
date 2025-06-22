package main

func rk4_step(f func(float64, float64) float64, t, y, h float64) float64 {
	k1 := h * f(t, y)
	k2 := h * f(t+0.5*h, y+0.5*k1)
	k3 := h * f(t+0.5*h, y+0.5*k2)
	k4 := h * f(t+h, y+k3)

	return y + (k1+2*k2+2*k3+k4)/6
}

func newton_step(f func(float64, float64) float64, t, y, h float64) float64 {
	return y + h*f(t, y)
}

func verlet_step(f func(float64, float64) float64, t, y, h float64) float64 {
	return y + h*f(t+h/2, y+h/2*f(t, y))
}
