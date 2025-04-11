package throttle_test

import (
	"github.com/rickb777/expect"
	"github.com/rickb777/goscrape2/download/throttle"
	"testing"
	"time"
)

func TestThrottle(t *testing.T) {
	for _, minimum := range []time.Duration{0, time.Millisecond} {
		th := throttle.New(minimum, 6*time.Second, 2*time.Second)

		expect.Bool(th.IsNormal()).Info(minimum).ToBeTrue(t)

		th.SlowDown()
		expect.Number(th.Delay()).Info(minimum).ToBe(t, 6*time.Second)

		th.SlowDown()
		expect.Number(th.Delay()).Info(minimum).ToBe(t, 8*time.Second)

		th.SlowDown()
		expect.Number(th.Delay()).Info(minimum).ToBe(t, 10*time.Second)

		th.SpeedUp()
		expect.Number(th.Delay()).Info(minimum).ToBe(t, 8*time.Second)

		th.SpeedUp()
		expect.Number(th.Delay()).Info(minimum).ToBe(t, 6*time.Second)

		th.SpeedUp()
		expect.Number(th.Delay()).Info(minimum).ToBe(t, 4*time.Second)

		th.SpeedUp()
		expect.Number(th.Delay()).Info(minimum).ToBe(t, 2*time.Second)

		th.SpeedUp()
		expect.Number(th.Delay()).Info(minimum).ToBe(t, minimum)

		th.Reset()
		expect.Bool(th.IsNormal()).Info(minimum).ToBeTrue(t)

		th.SpeedUp()
		expect.Number(th.Delay()).Info(minimum).ToBe(t, minimum)
	}
}
