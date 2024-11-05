package download

import (
	"sync/atomic"
	"time"
)

type Throttle int64

const (
	// speed up gently
	throttleDecrement = 100 * time.Millisecond
	// slow down sharply
	throttleIncrement = 20 * throttleDecrement
)

func (t *Throttle) SlowDown() {
	atomic.AddInt64((*int64)(t), int64(throttleIncrement))
}

func (t *Throttle) SpeedUp() {
	a := (*int64)(t)
	for { // loop until altered
		oldValue := atomic.LoadInt64(a)
		newValue := oldValue - int64(throttleDecrement)
		if newValue < 0 {
			return
		}
		if atomic.CompareAndSwapInt64(a, oldValue, newValue) {
			return
		}
	}
}

func (t *Throttle) Sleep() {
	time.Sleep(time.Duration(atomic.LoadInt64((*int64)(t))))
}
