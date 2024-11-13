package download

import (
	"sync/atomic"
	"time"
)

type Throttle int64

const (
	twoSeconds = 2 * time.Second
	tenSeconds = 10 * time.Second
)

var (
	smallStep = twoSeconds
	bigStep   = tenSeconds
)

func (t *Throttle) SlowDown() {
	a := (*int64)(t)
	if !atomic.CompareAndSwapInt64(a, 0, int64(bigStep)) {
		atomic.AddInt64(a, int64(smallStep))
	}
}

func (t *Throttle) Reset() {
	a := (*int64)(t)
	atomic.StoreInt64(a, int64(0))
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
