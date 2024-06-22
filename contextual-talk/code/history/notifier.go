package history

import (
	"errors"
	"fmt"
	"time"
)

func longOperationNotifier(cancellationNotifier <-chan struct{}) error {
	for i := 0; i < bigNumber; i++ {
		select {
		case <-cancellationNotifier:
			return errors.New("deadline")
		default:
		}
	}
	return nil
}

func Notifier(timeout time.Duration) {
	notifier := make(chan struct{})
	go func() {
		time.Sleep(timeout)
		notifier <- struct{}{}
	}()
	fmt.Println(longOperationNotifier(notifier))
}
