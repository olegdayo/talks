package best_practices

import "context"

// Wrong.
type sWrong struct {
	ctx   context.Context
	field any
}

func (s sWrong) f(args ...any) {
	// ...
}

// Correct.
type sCorrect struct {
	field any
}

func (s sCorrect) f(ctx context.Context, args ...any) {
	// ...
}
