package download

import (
	"sync/atomic"
	"time"
)

type Throttle int64

const (
	halfSecond = 500 * time.Millisecond
	twoSeconds = 2 * time.Second
	tenSeconds = 10 * time.Second
)

func (t *Throttle) SlowDown() {
	a := (*int64)(t)
	if !atomic.CompareAndSwapInt64(a, 0, int64(tenSeconds)) {
		atomic.AddInt64(a, int64(twoSeconds))
	}
}

func (t *Throttle) SpeedUp() {
	a := (*int64)(t)
	for { // loop until altered
		oldValue := atomic.LoadInt64(a)
		if oldValue <= 0 {
			return
		}

		newValue := oldValue - int64(halfSecond)
		if newValue < 0 {
			newValue = 0
		}
		if atomic.CompareAndSwapInt64(a, oldValue, newValue) {
			return
		}
	}
}

func (t *Throttle) IsNormal() bool {
	a := (*int64)(t)
	return atomic.LoadInt64(a) == 0
}

func (t *Throttle) Sleep() {
	a := (*int64)(t)
	d := atomic.LoadInt64(a)
	time.Sleep(time.Duration(d))
}
