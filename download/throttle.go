package download

import (
	"sync/atomic"
	"time"
)

type Throttle int64

const (
	// speed up gently
	throttleDecrement = 10 * time.Millisecond
	// slow down sharply
	throttleIncrement = 100 * throttleDecrement
)

func (t *Throttle) SlowDown() {
	atomic.AddInt64((*int64)(t), int64(throttleIncrement))
}

func (t *Throttle) SpeedUp() {
	a := (*int64)(t)
	for { // loop until altered
		oldValue := atomic.LoadInt64(a)
		if oldValue == 0 {
			return
		}
		newValue := oldValue - int64(throttleDecrement)
		if newValue < 0 {
			newValue = 0
		}
		if atomic.CompareAndSwapInt64(a, oldValue, newValue) {
			return
		}
	}
}

func (t *Throttle) IsNormal() bool {
	d := atomic.LoadInt64((*int64)(t))
	return d == 0
}

func (t *Throttle) Sleep() {
	d := atomic.LoadInt64((*int64)(t))
	time.Sleep(time.Duration(d))
}
