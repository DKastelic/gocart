package main

import (
	"fmt"
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
	A TrajectoryType = iota
	B
	C1
	C2
	D1
	D2
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
	if vmax <= va && s > sa {
		trajectoryType = A
	} else if vmax > va && s <= sa {
		trajectoryType = B
	} else if vmax <= va && s <= sa && s > sv {
		trajectoryType = C1
	} else if vmax <= va && s <= sa && s <= sv {
		trajectoryType = C2
	} else if vmax > va && s > sa && s > sv {
		trajectoryType = D1
	} else {
		trajectoryType = D2
	}

	// calculate tj, ta, tv
	var tj, ta, tv float64
	switch trajectoryType {
	case B, C2:
		tj = math.Cbrt(s / (2 * jerk))
		ta = tj
		tv = 2 * tj
	case A, C1:
		tj = math.Sqrt(vmax / jerk)
		tv = s / vmax
		ta = tj
	case D1:
		tj = amax / jerk
		ta = vmax / amax
		tv = s / vmax
	case D2:
		tj = amax / jerk
		ta = 0.5 * (math.Sqrt((4*s*math.Pow(jerk, 2)+math.Pow(amax, 3))/(amax*math.Pow(jerk, 2))) - amax/jerk)
		tv = ta + tj
	}

	// calculate t1, t2, t3, t4, t5, t6, t7
	// Use arrays for t, a, v, p (indices 0..7)
	var t [8]float64
	var j [8]float64
	var a [8]float64
	var v [8]float64
	var p [8]float64

	// Time points
	t[0] = 0
	t[1] = tj
	t[2] = ta
	t[3] = tj + ta
	t[4] = tv
	t[5] = tj + tv
	t[6] = tv + ta
	t[7] = tv + ta + tj

	dtj := t[1] - t[0]
	dta := t[2] - t[1]
	dtv := t[4] - t[3]

	if start < end {
		jerk = mpc.max_jerk
	} else {
		jerk = -mpc.max_jerk
	}

	// Initial conditions
	j[0] = jerk
	a[0] = 0
	v[0] = 0
	p[0] = start

	// Phase 1: Increasing acceleration (jerk = j)
	j[1] = 0
	a[1] = jerk * dtj
	v[1] = 0.5 * jerk * dtj * dtj
	p[1] = p[0] + (1.0/6.0)*jerk*dtj*dtj*dtj

	// Phase 2: Constant acceleration (jerk = 0)
	j[2] = -jerk
	a[2] = a[1]
	v[2] = v[1] + a[1]*dta
	p[2] = p[1] + v[1]*dta + 0.5*a[1]*dta*dta

	// Phase 3: Decreasing acceleration (jerk = -j)
	j[3] = 0
	a[3] = a[2] - jerk*dtj
	v[3] = v[2] + a[2]*dtj - 0.5*jerk*dtj*dtj
	p[3] = p[2] + v[2]*dtj + 0.5*a[2]*dtj*dtj - (1.0/6.0)*jerk*dtj*dtj*dtj

	// Phase 4: Constant velocity (jerk = 0, acceleration = 0)
	j[4] = -jerk
	a[4] = 0
	v[4] = v[3]
	p[4] = p[3] + v[3]*dtv

	// Phase 5: Increasing deceleration
	j[5] = 0
	a[5] = -a[2]
	v[5] = v[2]
	p[5] = end - (p[2] - start)

	// Phase 6: Constant deceleration
	j[6] = jerk
	a[6] = -a[1]
	v[6] = v[1]
	p[6] = end - (p[1] - start)

	// Phase 7: Decreasing deceleration
	j[7] = 0
	a[7] = 0
	v[7] = 0
	p[7] = end

	fmt.Println("Trajectory will take ", t[7], "seconds to complete.")

	return Trajectory{
		start: start,
		end:   end,
		t:     t,
		state: [8]internalState{
			{p: p[0], v: v[0], a: a[0], j: j[0]},
			{p: p[1], v: v[1], a: a[1], j: j[1]},
			{p: p[2], v: v[2], a: a[2], j: j[2]},
			{p: p[3], v: v[3], a: a[3], j: j[3]},
			{p: p[4], v: v[4], a: a[4], j: j[4]},
			{p: p[5], v: v[5], a: a[5], j: j[5]},
			{p: p[6], v: v[6], a: a[6], j: j[6]},
			{p: p[7], v: v[7], a: a[7], j: j[7]},
		},
		t0: time.Now(),
	}
}

func (mpc *MovementPlanner) calculateStateInPhase(phase int, dt float64, trajectory Trajectory) internalState {
	initialState := trajectory.state[phase-1]

	// Calculate the position, velocity and acceleration at time t in the given phase
	return internalState{
		p: initialState.p + initialState.v*dt + 0.5*initialState.a*dt*dt + (1.0/6.0)*initialState.j*dt*dt*dt,
		v: initialState.v + initialState.a*dt + 0.5*initialState.j*dt*dt,
		a: initialState.a + initialState.j*dt,
		j: initialState.j,
	}
}

func (mpc *MovementPlanner) calculateStateAtTime(t float64, trajectory Trajectory) internalState {
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
			return mpc.calculateStateInPhase(phase, dt, trajectory)
		}
	}

	// This should never be reached, but return end state as fallback
	return trajectory.state[7]
}

func (mpc *MovementPlanner) GetCurrentTrajectoryPosition(trajectory Trajectory) float64 {
	t := time.Since(trajectory.t0).Seconds()
	return mpc.calculateStateAtTime(t, trajectory).p
}

func (mpc *MovementPlanner) GetCurrentTrajectoryVelocity(trajectory Trajectory) float64 {
	t := time.Since(trajectory.t0).Seconds()
	return mpc.calculateStateAtTime(t, trajectory).v
}

func (mpc *MovementPlanner) GetCurrentTrajectoryAcceleration(trajectory Trajectory) float64 {
	t := time.Since(trajectory.t0).Seconds()
	return mpc.calculateStateAtTime(t, trajectory).a
}

func (mpc *MovementPlanner) GetCurrentTrajectoryJerk(trajectory Trajectory) float64 {
	t := time.Since(trajectory.t0).Seconds()
	return mpc.calculateStateAtTime(t, trajectory).j
}
