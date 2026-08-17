package shared

import "time"

// Clock supplies the current time to application use cases.
type Clock interface {
	Now() time.Time
}

// SystemClock supplies the operating system time.
type SystemClock struct{}

// Now returns the current time.
func (SystemClock) Now() time.Time {
	return time.Now()
}
