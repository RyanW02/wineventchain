package broadcast

import (
	"errors"
	"time"
)

type ErrorWaitChannel struct {
	wc *WaitBroadcastChannel[error]
}

func NewErrorWaitChannel() *ErrorWaitChannel {
	return &ErrorWaitChannel{
		wc: NewWaitBroadcastChannel[error](),
	}
}

func (e *ErrorWaitChannel) Subscribe() chan chan error {
	return e.wc.Subscribe()
}

func (e *ErrorWaitChannel) Await(timeout time.Duration) error {
	errs, timedOut := e.wc.PublishAndWait(timeout)
	if timedOut {
		return nil
	}

	return errors.Join(errs...)
}
