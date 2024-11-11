package download

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestThrottle(t *testing.T) {
	var throttle Throttle
	assert.Equal(t, Throttle(0), throttle)

	throttle.SlowDown()
	assert.Equal(t, Throttle(thirtySeconds), throttle)

	throttle.SlowDown()
	assert.Equal(t, Throttle(32*time.Second), throttle)

	throttle.SlowDown()
	assert.Equal(t, Throttle(34*time.Second), throttle)

	throttle.Reset()
	assert.Equal(t, Throttle(0), throttle)
}
