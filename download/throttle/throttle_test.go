package throttle_test

import (
	"github.com/cornelk/goscrape/download/throttle"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestThrottle(t *testing.T) {
	for _, minimum := range []time.Duration{0, time.Millisecond} {
		th := throttle.New(minimum, 6*time.Second, 2*time.Second)

		assert.True(t, th.IsNormal(), "%s", minimum)

		th.SlowDown()
		assert.Equal(t, 6*time.Second, th.Delay(), "%s", minimum)

		th.SlowDown()
		assert.Equal(t, 8*time.Second, th.Delay(), "%s", minimum)

		th.SlowDown()
		assert.Equal(t, 10*time.Second, th.Delay(), "%s", minimum)

		th.SpeedUp()
		assert.Equal(t, 8*time.Second, th.Delay(), "%s", minimum)

		th.SpeedUp()
		assert.Equal(t, 6*time.Second, th.Delay(), "%s", minimum)

		th.SpeedUp()
		assert.Equal(t, 4*time.Second, th.Delay(), "%s", minimum)

		th.SpeedUp()
		assert.Equal(t, 2*time.Second, th.Delay(), "%s", minimum)

		th.SpeedUp()
		assert.Equal(t, minimum, th.Delay(), "%s", minimum)

		th.Reset()
		assert.True(t, th.IsNormal(), "%s", minimum)

		th.SpeedUp()
		assert.Equal(t, minimum, th.Delay(), "%s", minimum)
	}
}
