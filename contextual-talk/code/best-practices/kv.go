package best_practices

import (
	"context"
	"log/slog"
)

// Wrong.
func fWrongKey(ctx context.Context) {
	metadata := ctx.Value("x-header")
	slog.DebugContext(
		ctx, "got metadata",
		"metadata", metadata,
	)
}

// Correct.
type (
	firstKeyCorrect  struct{}
	secondKeyCorrect struct{}
)

func fCorrectKey(ctx context.Context) {
	firstValue := ctx.Value(firstKeyCorrect{})
	secondValue := ctx.Value(secondKeyCorrect{})

	slog.DebugContext(
		ctx, "got metadata",
		"first value", firstValue,
		"second value", secondValue,
	)
}

// Collision.
type (
	firstKeyCollision  = struct{}
	secondKeyCollision = struct{}
)

func fCollisionnKey(ctx context.Context) {
	firstValue := ctx.Value(firstKeyCollision{})
	secondValue := ctx.Value(secondKeyCollision{})

	// Collision: firstValue == second value!
	slog.DebugContext(
		ctx, "got metadata",
		"first value", firstValue,
		"second value", secondValue,
	)
}
