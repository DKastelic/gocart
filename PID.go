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
	// The minimum output
	MinOutput float64
	// The maximum output
	MaxOutput float64
	// The sample time
	SampleTime float64
}

// NewPID creates a new PID controller
func NewPID(kp, ki, kd, minOutput, maxOutput, sampleTime float64) *PID {
	return &PID{
		Kp:         kp,
		Ki:         ki,
		Kd:         kd,
		MinOutput:  minOutput,
		MaxOutput:  maxOutput,
		SampleTime: sampleTime,
	}
}

// Update updates the PID controller
func (pid *PID) Update(input float64) float64 {
	// Calculate the error
	error := pid.Setpoint - input

	// Calculate the integral of the error
	pid.Integral += error * pid.SampleTime

	// Calculate the derivative of the error
	derivative := (error - pid.PreviousError) / pid.SampleTime

	// Calculate the output
	pid.Output = pid.Kp*error + pid.Ki*pid.Integral + pid.Kd*derivative

	// Clamp the output to the minimum and maximum output
	if pid.Output < pid.MinOutput {
		pid.Output = pid.MinOutput
	} else if pid.Output > pid.MaxOutput {
		pid.Output = pid.MaxOutput
	}

	return pid.Output
}

// SetSetpoint sets the setpoint of the PID controller
func (pid *PID) SetSetpoint(setpoint float64) {
	pid.Setpoint = setpoint
	pid.Integral = 0
	pid.PreviousError = 0
}
