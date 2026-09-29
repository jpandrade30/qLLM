package appctx

import (
	"context"

	"qLLM/internal/access"
)

type key struct{}

func WithApp(ctx context.Context, app *access.App) context.Context {
	if app == nil {
		return ctx
	}
	return context.WithValue(ctx, key{}, app)
}

func App(ctx context.Context) *access.App {
	a, _ := ctx.Value(key{}).(*access.App)
	return a
}
