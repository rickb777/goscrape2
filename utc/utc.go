package utc

import "time"

// Now is a source of UTC time ticks and is pluggable for testing.
var Now = func() time.Time {
	return time.Now().UTC()
}
