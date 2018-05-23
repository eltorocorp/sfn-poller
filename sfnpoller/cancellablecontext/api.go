// Package cancellablecontext is a wrapper around context.WithCancel that exposes context.CancelFunc as a receiver.
//
// This is just a convenience package that makes passing a cancellable context to child services easier, as
// with this you only have to pass a CancellableContext to a child, rather than both a context.Context
// and a context.CancelFunc.
package cancellablecontext

import "context"

// API is a context that can broadcast cancellation.
type API struct {
	context.Context
	cancelFn context.CancelFunc
}

// New returns a reference to a new CancellableContext.
func New(parentCtx context.Context) *API {
	ctx, cancel := context.WithCancel(parentCtx)
	return &API{
		Context:  ctx,
		cancelFn: cancel,
	}
}

// Cancel calls the context.CancelFunc, initializing a cancellation of all listeners.
func (c *API) Cancel() {
	c.cancelFn()
}
