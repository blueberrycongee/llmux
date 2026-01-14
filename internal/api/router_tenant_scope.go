package api //nolint:revive // package name is intentional

import (
	"context"

	"github.com/blueberrycongee/llmux/internal/auth"
	"github.com/blueberrycongee/llmux/pkg/router"
)

func withRouterTenantScope(ctx context.Context) context.Context {
	if router.TenantScopeFromContext(ctx) != "" {
		return ctx
	}
	if authCtx := auth.GetAuthContext(ctx); authCtx != nil && authCtx.APIKey != nil {
		return router.WithTenantScope(ctx, authCtx.APIKey.ID)
	}
	return ctx
}
