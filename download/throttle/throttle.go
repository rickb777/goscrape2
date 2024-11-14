package throttle

import (
	"sync/atomic"
	"time"
)

// Throttle is a delay timer with a [Sleep] method for pausing loops. It is controlled
// by [SlowDown] and [Reset]. It is safe for use across multiple goroutines. It is lock-free.
// It has a linear back-off algorithm implemented by [SlowDown] and [Speedup].
//
// All methods in a nil *Throttle are no-op.
type Throttle struct {
	delay   atomic.Int64
	min     int64
	initial int64
	extra   int64
}

// New returns a new Throttle with the minimum, initial and extra values specified.
// This panics if minimum is less than zero or if initialStep is less than minimum.
func New(minimum, initialStep, extraStep time.Duration) *Throttle {
	if minimum < 0 {
		panic("negative minimum delay")
	}
	if initialStep < minimum {
		panic("initialStep must be greater than the minimum delay")
	}

	t := &Throttle{
		min:     int64(minimum),
		extra:   int64(extraStep),
		initial: int64(initialStep),
	}
	t.delay.Store(int64(minimum))
	return t
}

// SlowDown increases the pause imposed when [Sleep] is called. The first time this is used,
// the throttle increases its delay to the initial step. Subsequently, it adds the extra step.
// This provides a linear back-off (n.b. not exponential).
func (t *Throttle) SlowDown() {
	if t != nil {
		if !t.delay.CompareAndSwap(t.min, t.initial) {
			t.delay.Add(t.extra)
		}
	}
}

// SpeedUp decreases the pause imposed when [Sleep] is called by subtracting the extra step
// from the delay. It has no effect after the minimum delay is reached.
func (t *Throttle) SpeedUp() {
	if t != nil {
		d := t.delay.Load()
		if d > t.min {
			newValue := d - t.extra
			if newValue < t.min {
				newValue = t.min
			}
			t.delay.CompareAndSwap(d, newValue)
		}
	}
}

// Reset reverts the loop delay to its minimum value.
func (t *Throttle) Reset() {
	if t != nil {
		t.delay.Store(t.min)
	}
}

// IsNormal returns true when the throttle is at its minimum.
func (t *Throttle) IsNormal() bool {
	return t == nil || t.delay.Load() == t.min
}

// Delay gets the current delay duration.
func (t *Throttle) Delay() time.Duration {
	if t == nil {
		return 0
	}
	return time.Duration(t.delay.Load())
}

// Sleep pauses this goroutine for the current loop delay. If the delay is zero,
// Sleep behaves as a no-op.
func (t *Throttle) Sleep() {
	if t != nil {
		time.Sleep(time.Duration(t.delay.Load()))
	}
}
