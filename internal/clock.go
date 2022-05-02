package internal

import "time"

// Clock allows to mock time.Now() in tests.
type Clock interface {
	Now() time.Time
}

// SystemClock returns system time.
type SystemClock struct{}

// Now returns the current local time.
func (SystemClock) Now() time.Time {
	return time.Now()
}

// MockClock returns a canned time.
type MockClock time.Time

// Now returns the mocked time.
func (m MockClock) Now() time.Time {
	return time.Time(m)
}
