// Package wait copy by https://github.com/kubernetes/apimachinery/blob/master/pkg/util/wait/wait.go
// and custom
package wait

import (
	"errors"
	"time"
)

// ErrWaitTimeout is returned when the condition exited without success.
var ErrWaitTimeout = errors.New("timed out waiting for the condition")

// ConditionFunc returns true if the condition is satisfied, or an error
// if the loop should be aborted.
type ConditionFunc func() (done bool, err error)

// Poll tries a condition func until it returns true, an error, or the timeout
// is reached.
//
// Poll always waits the interval before the run of 'condition'.
// 'condition' will always be invoked at least once.
//
// Some intervals may be missed if the condition takes too long or the time
// window is too short.
//
// If you want to Poll something forever, see PollInfinite.
func Poll(interval, timeout time.Duration, condition ConditionFunc) error {
	return pollInternal(poller(interval, timeout), condition)
}

// PollInfinite tries a condition func until it returns true or an error
//
// PollInfinite always waits the interval before the run of 'condition'.
//
// Some intervals may be missed if the condition takes too long or the time
// window is too short.
func PollInfinite(interval time.Duration, condition ConditionFunc) error {
	done := make(chan struct{})
	defer close(done)
	return PollUntil(interval, condition, done)
}

func pollInternal(wait WaitFunc, condition ConditionFunc) error {
	done := make(chan struct{})
	defer close(done)
	return WaitFor(wait, condition, done)
}

// PollUntil tries a condition func until it returns true, an error or stopCh is
// closed.
//
// PolUntil always waits interval before the first run of 'condition'.
// 'condition' will always be invoked at least once.
func PollUntil(interval time.Duration, condition ConditionFunc, stopCh <-chan struct{}) error {
	return WaitFor(poller(interval, 0), condition, stopCh)
}

// WaitFunc creates a channel that receives an item every time a test
// should be executed and is closed when the last test should be invoked.
type WaitFunc func(done <-chan struct{}) <-chan struct{}

// WaitFor continually checks 'fn' as driven by 'wait'.
//
// WaitFor gets a channel from 'wait()'', and then invokes 'fn' once for every value
// placed on the channel and once more when the channel is closed.
//
// If 'fn' returns an error the loop ends and that error is returned, and if
// 'fn' returns true the loop ends and nil is returned.
//
// ErrWaitTimeout will be returned if the channel is closed without fn ever
// returning true.
func WaitFor(wait WaitFunc, fn ConditionFunc, done <-chan struct{}) error {
	c := wait(done)
	for {
		_, open := <-c
		ok, err := fn()
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		if !open {
			break
		}
	}
	return ErrWaitTimeout
}

// poller returns a WaitFunc that will send to the channel every interval until
// timeout has elapsed and then closes the channel.
//
// Over very short intervals you may receive no ticks before the channel is
// closed. A timeout of 0 is interpreted as an infinity.
//
// Output ticks are not buffered. If the channel is not ready to receive an
// item, the tick is skipped.
func poller(interval, timeout time.Duration) WaitFunc {
	return WaitFunc(func(done <-chan struct{}) <-chan struct{} {
		ch := make(chan struct{})
		go func() {
			defer close(ch)
			tick := time.NewTicker(interval)
			defer tick.Stop()
			var after <-chan time.Time
			if timeout != 0 {
				timer := time.NewTimer(timeout)
				after = timer.C
				defer timer.Stop()
			}
			for {
				select {
				case <-tick.C:
					select {
					case ch <- struct{}{}:
					default:
					}
				case <-after:
					return
				case <-done:
					return
				}
			}
		}()
		return ch
	})
}
