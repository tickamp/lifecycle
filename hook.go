package lifecycle

import (
	"context"
)

// ContextHook is a context-aware hook.
type ContextHook = func(context.Context) error

// Hook is a naive hook.
type Hook = func() error

// ErrorHook is a hook aimed at receiving events and optionally transforming
// errors.
type ErrorHook = func(event Event) error
