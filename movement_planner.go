package main

import (
	"math"
	"time"
)

type MovementPlanner struct {
	max_jerk         float64
	max_acceleration float64
	max_velocity     float64
}

type internalState struct {
	p float64
	v float64
	a float64
	j float64
}

type Trajectory struct {
	start, end float64          // Start and end positions of the trajectory
	t          [8]float64       // Time points for trajectory phases
	state      [8]internalState // States at each phase end (position, velocity, acceleration)
	t0         time.Time        // Start time of the trajectory
}

type TrajectoryType int

const (
	VelocityLimited TrajectoryType = iota
	JerkLimited
	AccelerationLimitedWithMaxVelocity
	AccelerationLimitedWithoutMaxVelocity
)

// NewMovementPlanner creates a new MPC instance with the given parameters
func NewMovementPlanner(max_jerk, max_acceleration, max_velocity float64) *MovementPlanner {
	return &MovementPlanner{
		max_jerk:         max_jerk,
		max_acceleration: max_acceleration,
		max_velocity:     max_velocity,
	}
}

func (mpc *MovementPlanner) CalculateTrajectory(start, end float64) Trajectory {
	s := math.Abs(end - start)

	// rename parameters to match the paper
	jerk := mpc.max_jerk
	amax := mpc.max_acceleration
	vmax := mpc.max_velocity

	// Calculate va, sa, sv
	va := math.Pow(amax, 2) / jerk
	sa := 2 * math.Pow(amax, 3) / math.Pow(jerk, 2)

	var sv float64
	if vmax*jerk < math.Pow(amax, 2) {
		sv = 2 * vmax * math.Sqrt(vmax/jerk)
	} else {
		sv = vmax * (vmax/amax + amax/jerk)
	}

	// establish which case for the trajectory we are in
	var trajectoryType TrajectoryType
	if vmax <= va && s > sa || vmax <= va && s <= sa && s > sv {
		trajectoryType = VelocityLimited
	} else if vmax > va && s <= sa || vmax <= va && s <= sa && s <= sv {
		trajectoryType = JerkLimited
	} else if vmax > va && s > sa && s > sv {
		trajectoryType = AccelerationLimitedWithMaxVelocity
	} else {
		trajectoryType = AccelerationLimitedWithoutMaxVelocity
	}

	// calculate tj, ta, tv
	var tj, ta, tv float64
	switch trajectoryType {
	case JerkLimited:
		tj = math.Cbrt(s / (2 * jerk))
		ta = tj
		tv = 2 * tj
	case VelocityLimited:
		tj = math.Sqrt(vmax / jerk)
		tv = s / vmax
		ta = tj
	case AccelerationLimitedWithMaxVelocity:
		tj = amax / jerk
		ta = vmax / amax
		tv = s / vmax
	case AccelerationLimitedWithoutMaxVelocity:
		tj = amax / jerk
		ta = 0.5 * (math.Sqrt((4*s*math.Pow(jerk, 2)+math.Pow(amax, 3))/(amax*math.Pow(jerk, 2))) - amax/jerk)
		tv = ta + tj
	}

	// calculate t1, t2, t3, t4, t5, t6, t7
	var t [8]float64

	// Time points
	t[0] = 0
	t[1] = tj
	t[2] = ta
	t[3] = tj + ta
	t[4] = tv
	t[5] = tj + tv
	t[6] = tv + ta
	t[7] = tv + ta + tj

	if start < end {
		jerk = mpc.max_jerk
	} else {
		jerk = -mpc.max_jerk
	}

	trajectory := Trajectory{
		start: start,
		end:   end,
		t:     t,
		state: [8]internalState{
			{j: jerk, p: start, v: 0, a: 0}, // Initial state
		},
		t0: time.Now(),
	}
	for i := 1; i < 8; i++ {
		trajectory.state[i] = trajectory.calculateStateInPhase(i, t[i]-t[i-1])
		// fix jerk
		if i == 6 {
			trajectory.state[i].j = jerk
		} else if i == 2 || i == 4 {
			trajectory.state[i].j = -jerk
		} else {
			trajectory.state[i].j = 0
		}
	}

	return trajectory
}

// EmergencyStop creates a trajectory that brings the cart to a complete stop
// as quickly as possible from its current state in the given trajectory
func (mpc *MovementPlanner) EmergencyStop(currentTrajectory *Trajectory) Trajectory {
	// Get current state
	currentTime := time.Since(currentTrajectory.t0).Seconds()
	currentState := currentTrajectory.calculateStateAtTime(currentTime)

	// If we're already in the deceleration phase, just return the current trajectory
	if currentTime > currentTrajectory.t[4] {
		return *currentTrajectory
	}

	// Calculate emergency stop trajectory using maximum deceleration
	// We need to bring velocity and acceleration to zero
	jerk := mpc.max_jerk
	amax := mpc.max_acceleration

	// Determine direction - decelerate opposite to current velocity
	if currentState.v > 0 {
		jerk = -mpc.max_jerk
		amax = -mpc.max_acceleration
	}

	// If we're already decelerating in the right direction, continue
	// If we're accelerating in the wrong direction, we need to reverse
	var phase1_jerk, phase2_jerk, phase3_jerk float64
	var t1, t2, t3 float64

	// Phase 1: Adjust acceleration to maximum deceleration
	if (currentState.v > 0 && currentState.a > -mpc.max_acceleration) ||
		(currentState.v < 0 && currentState.a < mpc.max_acceleration) {
		phase1_jerk = jerk
		t1 = math.Abs(amax-currentState.a) / mpc.max_jerk
	} else {
		t1 = 0
		phase1_jerk = 0
	}

	// Calculate state after phase 1
	state1 := internalState{
		p: currentState.p + currentState.v*t1 + 0.5*currentState.a*t1*t1 + (1.0/6.0)*phase1_jerk*t1*t1*t1,
		v: currentState.v + currentState.a*t1 + 0.5*phase1_jerk*t1*t1,
		a: currentState.a + phase1_jerk*t1,
		j: 0,
	}

	// Phase 2: Constant maximum deceleration until we need to ease off
	phase2_jerk = 0
	velocityToStop := math.Abs(state1.v)
	timeToEaseOff := math.Abs(state1.a) / mpc.max_jerk
	velocityDuringEaseOff := 0.5 * math.Abs(state1.a) * timeToEaseOff

	if velocityToStop > velocityDuringEaseOff {
		t2 = (velocityToStop - velocityDuringEaseOff) / math.Abs(state1.a)
	} else {
		t2 = 0
	}

	// Calculate state after phase 2
	state2 := internalState{
		p: state1.p + state1.v*t2 + 0.5*state1.a*t2*t2,
		v: state1.v + state1.a*t2,
		a: state1.a,
		j: 0,
	}

	// Phase 3: Ease off acceleration to zero
	phase3_jerk = -jerk // Opposite direction to reduce deceleration to zero
	t3 = math.Abs(state2.a) / mpc.max_jerk

	// Calculate final state
	finalState := internalState{
		p: state2.p + state2.v*t3 + 0.5*state2.a*t3*t3 + (1.0/6.0)*phase3_jerk*t3*t3*t3,
		v: state2.v + state2.a*t3 + 0.5*phase3_jerk*t3*t3,
		a: 0,
		j: 0,
	}

	// Create trajectory with 4 phases (current state + 3 stop phases)
	var t [8]float64
	t[0] = 0
	t[1] = t1
	t[2] = t1 + t2
	t[3] = t1 + t2 + t3
	// Fill remaining times with the final time (stationary)
	for i := 4; i < 8; i++ {
		t[i] = t[3]
	}

	trajectory := Trajectory{
		start: currentState.p,
		end:   finalState.p,
		t:     t,
		state: [8]internalState{
			{j: phase1_jerk, p: currentState.p, v: currentState.v, a: currentState.a},
			{j: phase2_jerk, p: state1.p, v: state1.v, a: state1.a},
			{j: phase3_jerk, p: state2.p, v: state2.v, a: state2.a},
			{j: 0, p: finalState.p, v: finalState.v, a: finalState.a},
			{j: 0, p: finalState.p, v: 0, a: 0}, // Ensure we're completely stopped
			{j: 0, p: finalState.p, v: 0, a: 0},
			{j: 0, p: finalState.p, v: 0, a: 0},
			{j: 0, p: finalState.p, v: 0, a: 0},
		},
		t0: time.Now(),
	}

	return trajectory
}

func (trajectory Trajectory) calculateStateInPhase(phase int, dt float64) internalState {
	initialState := trajectory.state[phase-1]

	// Calculate the position, velocity and acceleration at time t in the given phase
	return internalState{
		p: initialState.p + initialState.v*dt + 0.5*initialState.a*dt*dt + (1.0/6.0)*initialState.j*dt*dt*dt,
		v: initialState.v + initialState.a*dt + 0.5*initialState.j*dt*dt,
		a: initialState.a + initialState.j*dt,
		j: initialState.j,
	}
}

func (trajectory Trajectory) calculateStateAtTime(t float64) internalState {
	// Before the trajectory starts, return the initial state
	if t <= 0 {
		return trajectory.state[0]
	}

	// After the trajectory ends, return the end state
	if t > trajectory.t[7] {
		return trajectory.state[7]
	}

	// Find which phase we're in and calculate the state
	for phase := 1; phase <= 7; phase++ {
		if t <= trajectory.t[phase] {
			dt := t - trajectory.t[phase-1]
			return trajectory.calculateStateInPhase(phase, dt)
		}
	}

	// This should never be reached, but return end state as fallback
	return trajectory.state[7]
}

func (trajectory Trajectory) GetCurrentPosition() float64 {
	t := time.Since(trajectory.t0).Seconds()
	return trajectory.calculateStateAtTime(t).p
}

func (trajectory Trajectory) GetCurrentVelocity() float64 {
	t := time.Since(trajectory.t0).Seconds()
	return trajectory.calculateStateAtTime(t).v
}

func (trajectory Trajectory) GetCurrentAcceleration() float64 {
	t := time.Since(trajectory.t0).Seconds()
	return trajectory.calculateStateAtTime(t).a
}

func (trajectory Trajectory) GetCurrentJerk() float64 {
	t := time.Since(trajectory.t0).Seconds()
	return trajectory.calculateStateAtTime(t).j
}

func (trajectory Trajectory) IsFinished() bool {
	t := time.Since(trajectory.t0).Seconds()
	return t >= trajectory.t[7]
}
