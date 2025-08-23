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
	t float64 // time since t0
	p float64
	v float64
	a float64
	j float64
}

/*
phases:

Stopping phases:

0: Increasing deceleration
1: Constant deceleration
2: Decreasing deceleration

Point to point phases:

3: Increasing acceleration
4: Constant acceleration
5: Decreasing acceleration
6: Constant velocity
7: Increasing deceleration
8: Constant deceleration
9: Decreasing deceleration
10: Final state
*/
type Trajectory struct {
	end   float64          // Start and end positions of the trajectory
	state [8]internalState // States at each phase end (position, velocity, acceleration)
	t0    time.Time        // Start time of the trajectory

	tjStop1, taStop, tjStop2, tj, ta, tv float64

	isStopping bool // Whether this trajectory is a stopping trajectory
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

func (mpc *MovementPlanner) GetStationaryTrajectory(point float64) *Trajectory {
	// Create a stationary trajectory at the given point
	return &Trajectory{
		end: point,
		t0:  time.Now(),
		state: [8]internalState{
			{t: 0, p: point, v: 0, a: 0, j: 0},
			{t: 0, p: point, v: 0, a: 0, j: 0},
			{t: 0, p: point, v: 0, a: 0, j: 0},
			{t: 0, p: point, v: 0, a: 0, j: 0},
			{t: 0, p: point, v: 0, a: 0, j: 0},
			{t: 0, p: point, v: 0, a: 0, j: 0},
			{t: 0, p: point, v: 0, a: 0, j: 0},
			{t: 0, p: point, v: 0, a: 0, j: 0},
		},
		tjStop1: 0,
		taStop:  0,
		tjStop2: 0,
		tj:      0,
		ta:      0,
		tv:      0,
	}
}

func (mpc *MovementPlanner) CalculatePointToPointTrajectory(start float64, end float64) *Trajectory {
	t0 := time.Now()

	s := math.Abs(end - start)

	// rename parameters to match the paper
	jerk := mpc.max_jerk
	amax := mpc.max_acceleration
	vmax := mpc.max_velocity

	// Calculate va, sa, sv
	va := amax * amax / jerk
	sa := 2 * amax * amax * amax / (jerk * jerk)

	var sv float64
	if vmax*jerk < amax*amax {
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
		ta = 0
		tv = 0
	case VelocityLimited:
		tj = math.Sqrt(vmax / jerk)
		ta = 0
		tv = s/vmax - 2*tj
	case AccelerationLimitedWithMaxVelocity:
		tj = amax / jerk
		ta = vmax/amax - tj
		tv = s/vmax - 2*tj - ta
	case AccelerationLimitedWithoutMaxVelocity:
		tj = amax / jerk
		ta = 0.5*math.Sqrt((4*s*math.Pow(jerk, 2)+math.Pow(amax, 3))/(amax*math.Pow(jerk, 2))) - 1.5*tj
		tv = 0
	}

	if start < end {
		jerk = mpc.max_jerk
	} else {
		jerk = -mpc.max_jerk
	}

	initialState := internalState{
		t: 0,
		p: start,
		v: 0,
		a: 0,
		j: 0,
	}

	afterIncreasingAcceleration := initialState.moveStateForward(tj, jerk)
	afterConstantAcceleration := afterIncreasingAcceleration.moveStateForward(ta, 0)
	afterDecreasingAcceleration := afterConstantAcceleration.moveStateForward(tj, -jerk)
	afterConstantVelocity := afterDecreasingAcceleration.moveStateForward(tv, 0)
	afterIncreasingDeceleration := afterConstantVelocity.moveStateForward(tj, -jerk)
	afterConstantDeceleration := afterIncreasingDeceleration.moveStateForward(ta, 0)
	afterDecreasingDeceleration := afterConstantDeceleration.moveStateForward(tj, jerk)

	// Fix the final state's jerk to zero
	afterDecreasingDeceleration.moveStateForward(0, 0)

	tr := &Trajectory{
		end: end,
		t0:  t0,
		state: [8]internalState{
			initialState,
			afterIncreasingAcceleration,
			afterConstantAcceleration,
			afterDecreasingAcceleration,
			afterConstantVelocity,
			afterIncreasingDeceleration,
			afterConstantDeceleration,
			afterDecreasingDeceleration,
		},
		tj: tj,
		ta: ta,
		tv: tv,
	}
	return tr
}
func (mpc *MovementPlanner) CalculateStoppingTrajectory(previousTrajectory *Trajectory) *Trajectory {
	// If the previous trajectory is a stopping trajectory, calculate from it
	if previousTrajectory.isStopping {
		return mpc.calculateStoppingTrajectoryFromStoppingTrajectory(previousTrajectory)
	}

	// If the previous trajectory is a point-to-point trajectory, calculate from it
	return mpc.calculateStoppingTrajectoryFromPointToPointTrajectory(previousTrajectory)
}

func (mpc *MovementPlanner) calculateStoppingTrajectoryFromStoppingTrajectory(previousTrajectory *Trajectory) *Trajectory {
	t0 := time.Now()

	// calculate the stopping times

	previousTrajectoryTime := time.Since(previousTrajectory.t0).Seconds()
	// tjStop1 is to bring us to maximum deceleration,
	// tjStop2 is to bring acceleration and velocity both back to zero
	var tjStop1, taStop, tjStop2 float64

	if previousTrajectoryTime <= previousTrajectory.state[1].t {
		// we are in the first braking phase (braking jerk 1)
		remainingTime := previousTrajectory.state[1].t - previousTrajectoryTime

		tjStop1 = remainingTime
		taStop = previousTrajectory.taStop
		tjStop2 = previousTrajectory.tjStop2
	} else if previousTrajectoryTime <= previousTrajectory.state[2].t {
		// we are in the second braking phase (constant braking deceleration)
		remainingTime := previousTrajectory.state[2].t - previousTrajectoryTime

		tjStop1 = 0
		taStop = remainingTime
		tjStop2 = previousTrajectory.tjStop2
	} else if previousTrajectoryTime <= previousTrajectory.state[3].t {
		// we are in the third braking phase (braking jerk 2)
		remainingTime := previousTrajectory.state[3].t - previousTrajectoryTime

		tjStop1 = 0
		taStop = 0
		tjStop2 = remainingTime
	} else {
		tjStop1 = 0
		taStop = 0
		tjStop2 = 0
	}

	// calculate our final position after stopping
	initialState := previousTrajectory.calculateStateAtTime(previousTrajectoryTime)
	initialState.t = 0
	var jStop float64
	if initialState.v > 0 {
		jStop = mpc.max_jerk
	} else {
		jStop = -mpc.max_jerk
	}

	afterFirstBrakingJerk := initialState.moveStateForward(tjStop1, -jStop)
	afterConstantBrakingDeceleration := afterFirstBrakingJerk.moveStateForward(taStop, 0)
	afterSecondBrakingJerk := afterConstantBrakingDeceleration.moveStateForward(tjStop2, jStop)

	// Fix the final state's jerk to zero
	afterSecondBrakingJerk.moveStateForward(0, 0)

	tr := &Trajectory{
		end: afterSecondBrakingJerk.p, // end position is the final position after stopping
		t0:  t0,
		state: [8]internalState{
			initialState,
			afterFirstBrakingJerk,
			afterConstantBrakingDeceleration,
			afterSecondBrakingJerk,
			afterSecondBrakingJerk,
			afterSecondBrakingJerk,
			afterSecondBrakingJerk,
			afterSecondBrakingJerk,
		},
		tjStop1: tjStop1,
		tjStop2: tjStop2,

		isStopping: true,
	}
	return tr
}

func (mpc *MovementPlanner) calculateStoppingTrajectoryFromPointToPointTrajectory(previousTrajectory *Trajectory) *Trajectory {
	t0 := time.Now()

	// calculate the stopping timesW

	previousTrajectoryTime := time.Since(previousTrajectory.t0).Seconds()
	// tjStop1 is to bring us to maximum deceleration,
	// tjStop2 is to bring acceleration and velocity both back to zero
	var tjStop1, taStop, tjStop2 float64

	if previousTrajectoryTime <= previousTrajectory.state[1].t {
		// we are in the first phase (increasing acceleration)
		tj := previousTrajectoryTime

		tjStop1 = 2 * tj
		taStop = 0
		tjStop2 = tj
	} else if previousTrajectoryTime <= previousTrajectory.state[2].t {
		// we are in the second phase (constant acceleration)
		tj := previousTrajectory.tj
		ta := previousTrajectoryTime - previousTrajectory.state[1].t

		tjStop1 = 2 * tj
		taStop = ta
		tjStop2 = tj
	} else if previousTrajectoryTime <= previousTrajectory.state[3].t {
		// we are in the third phase (decreasing acceleration)
		tj := previousTrajectory.tj
		remainingTime := previousTrajectory.state[3].t - previousTrajectoryTime

		tjStop1 = remainingTime + tj
		taStop = previousTrajectory.ta
		tjStop2 = tj
	} else if previousTrajectoryTime <= previousTrajectory.state[4].t {
		// we are in the fourth phase (constant velocity)
		// move the remainging phases to start now,
		// as we already know that they are optimal for stopping from a given velocity
		tj := previousTrajectory.tj

		tjStop1 = tj
		taStop = previousTrajectory.ta
		tjStop2 = tj
	} else if previousTrajectoryTime <= previousTrajectory.state[5].t {
		// we are in the fifth phase (increasing deceleration)
		tj := previousTrajectory.tj
		remainingTime := previousTrajectory.state[5].t - previousTrajectoryTime

		tjStop1 = remainingTime
		taStop = previousTrajectory.ta
		tjStop2 = tj
	} else if previousTrajectoryTime <= previousTrajectory.state[6].t {
		// we are in the sixth phase (constant deceleration)
		tj := previousTrajectory.tj
		remainingTime := previousTrajectory.state[6].t - previousTrajectoryTime

		tjStop1 = 0
		taStop = remainingTime
		tjStop2 = tj
	} else if previousTrajectoryTime <= previousTrajectory.state[7].t {
		// we are in the seventh phase (decreasing deceleration)
		remainingTime := previousTrajectory.state[7].t - previousTrajectoryTime

		tjStop1 = 0
		taStop = 0
		tjStop2 = remainingTime
	} else {
		tjStop1 = 0
		taStop = 0
		tjStop2 = 0
	}

	// calculate our final position after stopping
	initialState := previousTrajectory.calculateStateAtTime(previousTrajectoryTime)
	initialState.t = 0
	var jStop float64
	if initialState.v > 0 {
		jStop = mpc.max_jerk
	} else {
		jStop = -mpc.max_jerk
	}

	afterFirstBrakingJerk := initialState.moveStateForward(tjStop1, -jStop)
	afterConstantBrakingDeceleration := afterFirstBrakingJerk.moveStateForward(taStop, 0)
	afterSecondBrakingJerk := afterConstantBrakingDeceleration.moveStateForward(tjStop2, jStop)

	// Fix the final state's jerk to zero
	afterSecondBrakingJerk.moveStateForward(0, 0)

	tr := &Trajectory{
		end: afterSecondBrakingJerk.p, // end position is the final position after stopping
		t0:  t0,
		state: [8]internalState{
			initialState,
			afterFirstBrakingJerk,
			afterConstantBrakingDeceleration,
			afterSecondBrakingJerk,
			afterSecondBrakingJerk,
			afterSecondBrakingJerk,
			afterSecondBrakingJerk,
			afterSecondBrakingJerk,
		},
		tjStop1: tjStop1,
		tjStop2: tjStop2,

		isStopping: true,
	}
	return tr
}

func (trajectory Trajectory) calculateStateInPhase(phase int, dt float64) internalState {
	initialState := trajectory.state[phase-1]

	// Calculate the position, velocity and acceleration at time t in the given phase
	return initialState.moveStateForward(dt, initialState.j)
}

func (initialState *internalState) moveStateForward(dt, jerk float64) internalState {
	// set the initial state's jerk
	initialState.j = jerk

	// Calculate the position, velocity and acceleration at time t in the given phase
	return internalState{
		t: initialState.t + dt,
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
	if t > trajectory.state[7].t {
		return trajectory.state[7]
	}

	// Find which phase we're in and calculate the state
	for phase := 1; phase <= 7; phase++ {
		if t <= trajectory.state[phase].t {
			dt := t - trajectory.state[phase-1].t
			return trajectory.calculateStateInPhase(phase, dt)
		}
	}

	// This should never be reached, but return end state as fallback
	return trajectory.state[7]
}

func (trajectory Trajectory) GetCurrentState() internalState {
	t := time.Since(trajectory.t0).Seconds()
	return trajectory.calculateStateAtTime(t)
}

func (trajectory Trajectory) GetCurrentPosition() float64 {
	return trajectory.GetCurrentState().p
}

func (trajectory Trajectory) GetCurrentVelocity() float64 {
	return trajectory.GetCurrentState().v
}

func (trajectory Trajectory) GetCurrentAcceleration() float64 {
	return trajectory.GetCurrentState().a
}

func (trajectory Trajectory) GetCurrentJerk() float64 {
	return trajectory.GetCurrentState().j
}

func (trajectory Trajectory) IsFinished() bool {
	return time.Since(trajectory.t0).Seconds() >= trajectory.state[7].t
}

func (trajectory Trajectory) GetBounds() (float64, float64) {
	start := trajectory.state[0].p
	afterBraking := trajectory.state[3].p
	end := trajectory.state[7].p
	min := min(start, afterBraking, end)
	max := max(start, afterBraking, end)
	return min, max
}
