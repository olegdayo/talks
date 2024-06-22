package best_practices

import (
	"context"
	"log/slog"
)

// Wrong.
func fWrong(ctx context.Context) {
	optStr, ok := ctx.Value("some-key").(string)
	if !ok {
		// ...
	}

	slog.DebugContext(
		ctx, "got str",
		"str", optStr,
	)
	// ...
}

// Correct.
func fCorrect(ctx context.Context, optStr *string) {
	if optStr == nil {
		// ...
	}

	slog.DebugContext(
		ctx, "got str",
		"str", *optStr,
	)
	// ...
}
