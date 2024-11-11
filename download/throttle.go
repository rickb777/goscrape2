package download

import (
	"sync/atomic"
	"time"
)

type Throttle int64

const (
	twoSeconds    = 2 * time.Second
	thirtySeconds = 30 * time.Second
)

var MinimumThrottle time.Duration = 0

func (t *Throttle) SlowDown() {
	a := (*int64)(t)
	if !atomic.CompareAndSwapInt64(a, int64(MinimumThrottle), int64(thirtySeconds)) {
		atomic.AddInt64(a, int64(twoSeconds))
	}
}

func (t *Throttle) Reset() {
	a := (*int64)(t)
	atomic.StoreInt64(a, int64(MinimumThrottle))
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
