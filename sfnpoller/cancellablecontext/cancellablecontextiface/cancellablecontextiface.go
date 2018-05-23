// Package cancellablecontextiface represents a context that supports cancellation.
package cancellablecontextiface

import "context"

// Context represents a context that can be cancelled.
type Context interface {
	context.Context
	Cancel()
}
