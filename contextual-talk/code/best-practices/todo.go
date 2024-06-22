package best_practices

import "context"

func f(ctx context.Context) {
	// ...
}

// Wrong.
func callerWrong() {
	f(nil)
}

// Correct.
func callerCorrect() {
	f(context.TODO())
}
