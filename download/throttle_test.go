package download

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestThrottle(t *testing.T) {
	var throttle Throttle
	assert.Equal(t, Throttle(0), throttle)

	throttle.SlowDown()
	assert.Equal(t, Throttle(tenSeconds), throttle)

	throttle.SlowDown()
	assert.Equal(t, Throttle(tenSeconds+twoSeconds), throttle)

	throttle.SlowDown()
	assert.Equal(t, Throttle(tenSeconds+twoSeconds+twoSeconds), throttle)

	throttle.Reset()
	assert.Equal(t, Throttle(0), throttle)
}
