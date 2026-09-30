package appctx

import (
	"context"

	"qLLM/internal/access"
)

type key struct{}

// WithApp implements runtime behavior for this package.
func WithApp(ctx context.Context, app *access.App) context.Context {
	if app == nil {
		return ctx
	}
	return context.WithValue(ctx, key{}, app)
}

// App implements runtime behavior for this package.
func App(ctx context.Context) *access.App {
	a, _ := ctx.Value(key{}).(*access.App)
	return a
}
