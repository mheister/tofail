// Copyright (c) 2024 Marcel Heistermann

package timeout

import "time"

type TimerFactory interface {
	NewTimer(timeout time.Duration) <-chan time.Time
}

func GetDefaultTimerFactory() TimerFactory {
	return defaultTimerFactory{}
}

type defaultTimerFactory struct {}

func (defaultTimerFactory) NewTimer(timeout time.Duration) <-chan time.Time {
	return time.NewTimer(timeout).C
}
