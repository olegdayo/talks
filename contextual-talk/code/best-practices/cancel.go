package best_practices

import "context"

// Wrong.
func fNoCancel(ctx context.Context) {
	ctx, _ = context.WithCancel(ctx)
	// ...
}

// Correct.
func fCancel(ctx context.Context) {
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)
	defer cancel()
	// ...
}
