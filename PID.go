package main

type PID struct {
	// The proportional gain
	Kp float64
	// The integral gain
	Ki float64
	// The derivative gain
	Kd float64
	// The previous error
	PreviousError float64
	// The integral of the error
	Integral float64
	// The setpoint
	Setpoint float64
	// The output
	Output float64
	// The sample time
	SampleTime float64
	// The maximum output (minimum is -MaxOutput)
	MaxOutput float64
}

// NewPID creates a new PID controller
func NewPID(kp, ki, kd, sampleTime, maxOutput float64) *PID {
	return &PID{
		Kp:         kp,
		Ki:         ki,
		Kd:         kd,
		SampleTime: sampleTime,
		MaxOutput:  maxOutput,
	}
}

// Update updates the PID controller
func (pid *PID) Update(input float64) float64 {
	// Calculate the error
	err := pid.Setpoint - input

	// Calculate the integral of the error
	pid.Integral += err * pid.SampleTime

	// Calculate the derivative of the error
	derivative := (err - pid.PreviousError) / pid.SampleTime

	// Calculate the output
	pid.Output = pid.Kp*err + pid.Ki*pid.Integral + pid.Kd*derivative

	// Clamp the output to the maximum and minimum values
	if pid.Output > pid.MaxOutput {
		pid.Output = pid.MaxOutput
	} else if pid.Output < -pid.MaxOutput {
		pid.Output = -pid.MaxOutput
	}

	// Update the previous error
	pid.PreviousError = err

	return pid.Output
}

// SetSetpoint sets the setpoint of the PID controller
func (pid *PID) SetSetpoint(setpoint float64) {
	pid.Setpoint = setpoint
	// pid.PreviousError = 0
}
