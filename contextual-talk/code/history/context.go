package history

import (
	"context"
	"errors"
	"fmt"
	"time"
)

func longOperationContext(ctx context.Context) error {
	for i := 0; i < bigNumber; i++ {
		select {
		case <-ctx.Done():
			return errors.New("deadline")
		default:
		}
	}
	return nil
}

func Context(timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	fmt.Println(longOperationContext(ctx))
}
