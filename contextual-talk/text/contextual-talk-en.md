# Functionality

```go
type Context interface {
	// Deadline returns context's deadline.
	Deadline() (deadline time.Time, ok bool)
	// Done returns channel which locks until the context is finished.
	// The types of finish usually are:
	// 1. Context has been cancelled;
	// 2. Context's deadline has been exceeded;
	Done() <-chan struct{}
	// Err returns cause of context finish.
	Err() error
	// Returns some stored value.
	Value(key any) any
}
```

# History

## Before

```go
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

func ntfr(timeout time.Duration) {
	notifier := make(chan struct{})
	go func() {
		time.Sleep(timeout)
		notifier <- struct{}{}
	}()
	fmt.Println(longOperationNotifier(notifier))
}
```

## Now

```go
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

func ctx(timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	fmt.Println(longOperationContext(ctx))
}
```

# Types and Their Implementation

## Empty Context

```go
type emptyContext struct{}

func (emptyContext) Deadline() (deadline time.Time, ok bool) {
	return time.Time{}, false
}

func (emptyContext) Done() <-chan struct{} {
	return nil
}

func (emptyContext) Err() error {
	return nil
}

func (emptyContext) Value(key any) any {
	return nil
}
```

## Cancel Context

```go
type cancelCtx struct {
	// Parent context.
	Context
	// Chan struct.
	done atomic.Value
	// Omit mutex for simplicity.
	children map[canceler]struct{}
	err, cause string
}

type canceler interface {
	cancel(removeFromParent bool, err, cause error)
	Done() <-chan struct{}
}
```

```go
var closedchan = make(chan struct{})

func init() {
	close(closedchan)
}

var cancelCtxKey int
```

```go
func WithCancel(parent Context) (
	ctx Context,
	cancel CancelFunc,
) {
	c := &cancelCtx{}
	c.propagateCancel(parent, c)

	return c, func() {
		c.cancel(true, Canceled, nil)
	}
}
```

```go
func (c *cancelCtx) Done() <-chan struct{} {
	d := c.done.Load()
	if d == nil {
		d = make(chan struct{})
		c.done.Store(d)
	}
	return d.(chan struct{})
}

func (c *cancelCtx) Err() error {
	return c.err
}

func (c *cancelCtx) Value(key any) any {
	if key == &cancelCtxKey {
		return c
	}
	return value(c.Context, key)
}
```


```go
func (c *cancelCtx) propagateCancel(parent Context, child canceler) {
	c.Context = parent

	// Check for parent valid cancellation ...

	if p, ok := parentCancelCtx(parent); ok {
		// Parent is a *cancelCtx, or derives from one.
		if p.err != nil {
			// Parent has already been canceled.
			child.cancel(false, p.err, p.cause)
		} else {
			p.children[child] = struct{}{}
		}
		return
	}

	if a, ok := parent.(afterFuncer); ok {
		// parent implements an AfterFunc method.
		stop := a.AfterFunc(func() {
			child.cancel(false, parent.Err(), Cause(parent))
		})
		c.Context = stopCtx{
			Context: parent,
			stop:    stop,
		}
		return
	}

	go func() {
		select {
		case <-parent.Done():
			child.cancel(false, parent.Err(), Cause(parent))
		case <-child.Done():
		}
	}()
}
```

```go
func (c *cancelCtx) cancel(removeFromParent bool, err error, cause error) {
	if c.err != nil {
		// Already cancelled.
		return
	}
	c.err = err
	c.cause = cause

	d, _ := c.done.Load().(chan struct{})
	if d == nil {
		c.done.Store(closedchan)
	} else {
		close(d)
	}

	for child := range c.children {
		child.cancel(false, err, cause)
	}
	c.children = nil

	if removeFromParent {
		removeChild(c.Context, c)
	}
}
```

```go
func parentCancelCtx(parent Context) (*cancelCtx, bool) {
	done := parent.Done()
	if done == closedchan || done == nil {
		return nil, false
	}

	p, ok := parent.Value(&cancelCtxKey).(*cancelCtx)
	if !ok {
		return nil, false
	}

	pdone, _ := p.done.Load().(chan struct{})
	if pdone != done {
		return nil, false
	}

	return p, true
}
```

## Context without Cancel

```go
type withoutCancelCtx struct {
	c Context
}

func WithoutCancel(parent Context) Context {
	return withoutCancelCtx{parent}
}

func (withoutCancelCtx) Deadline() (deadline time.Time, ok bool) {
	return
}

func (withoutCancelCtx) Done() <-chan struct{} {
	return nil
}

func (withoutCancelCtx) Err() error {
	return nil
}

func (c withoutCancelCtx) Value(key any) any {
	return value(c, key)
}
```

## AfterFunc

```go
func AfterFunc(ctx Context, f func()) (stop func() bool) {
	a := &afterFuncCtx{
		f: f,
	}
	a.cancelCtx.propagateCancel(ctx, a)
	return func() bool {
		stopped := false
		a.once.Do(func() {
			stopped = true
		})
		if stopped {
			a.cancel(true, Canceled, nil)
		}
		return stopped
	}
}

type afterFuncer interface {
	AfterFunc(func()) func() bool
}

type afterFuncCtx struct {
	cancelCtx
	// Either starts running f or stops f from running.
	once sync.Once
	f    func()
}

func (a *afterFuncCtx) cancel(removeFromParent bool, err, cause error) {
	a.cancelCtx.cancel(false, err, cause)
	if removeFromParent {
		removeChild(a.Context, a)
	}
	a.once.Do(func() {
		go a.f()
	})
}
```

```go
func read(ctx context.Context, conn net.Conn, b []byte) (n int, err error) {
	stopc := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		conn.SetReadDeadline(time.Now())
		close(stopc)
	})
	n, err = conn.Read(b)

	// Cancel.
	if !stop() {
		// The AfterFunc was started.
		// Wait for it to complete, and reset the Conn's deadline.
		<-stopc
		conn.SetReadDeadline(time.Time{})
		return n, ctx.Err()
	}

	// Read successfully.
	return n, err
}
```

## Context with Deadline

```go
type timerCtx struct {
	cancelCtx
	// Under cancelCtx.mu.
	timer *time.Timer
	deadline time.Time
}

func WithDeadline(parent Context, d time.Time) (Context, CancelFunc) {
	if cur, ok := parent.Deadline(); ok && cur.Before(d) {
		// The current deadline is already sooner than the new one.
		return WithCancel(parent)
	}
	c := &timerCtx{
		deadline: d,
	}
	c.cancelCtx.propagateCancel(parent, c)
	dur := time.Until(d)
	if dur <= 0 {
		// Deadline has already passed.
		c.cancel(true, DeadlineExceeded, nil)
		return c, func() { c.cancel(false, Canceled, nil) }
	}

	if c.err == nil {
		c.timer = time.AfterFunc(dur, func() {
			c.cancel(true, DeadlineExceeded, nil)
		})
	}
	return c, func() { c.cancel(true, Canceled, nil) }
}
```

```go
func (c *timerCtx) Deadline() (deadline time.Time, ok bool) {
	return c.deadline, true
}

func (c *timerCtx) cancel(removeFromParent bool, err, cause error) {
	c.cancelCtx.cancel(false, err, cause)
	if removeFromParent {
		// Remove this timerCtx from its parent cancelCtx's children.
		removeChild(c.cancelCtx.Context, c)
	}

	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
}
```

## Context with Timeout

```go
func WithTimeout(parent Context, timeout time.Duration) (Context, CancelFunc) {
	return WithDeadline(parent, time.Now().Add(timeout))
}
```

## Context with Cause

```go
var Canceled error
var DeadlineExceeded error

func Cause(c Context) error {
	cc, ok := c.Value(&cancelCtxKey).(*cancelCtx)
	if ok {
		return cc.cause
	}
	return c.Err()
}

func WithCancelCause(parent Context) (ctx Context, cancel CancelCauseFunc) {
	c := withCancel(parent)
	return c, func(cause error) { c.cancel(true, Canceled, cause) }
}
```

## Context with Value

```go
type valueCtx struct {
	Context
	key any
	val any
}

func WithValue(parent Context, key, val any) Context {
	if !reflectlite.TypeOf(key).Comparable() {
		panic("key is not comparable")
	}
	return &valueCtx{parent, key, val}
}
```

```go
func (c *valueCtx) Value(key any) any {
	if c.key == key {
		return c.val
	}
	return value(c.Context, key)
}

func value(c Context, key any) any {
	for {
		switch ctx := c.(type) {
		case *valueCtx:
			if key == ctx.key {
				return ctx.val
			}
		case *cancelCtx:
			if key == &cancelCtxKey {
				return c
			}
		case withoutCancelCtx:
			if key == &cancelCtxKey {
				// This implements Cause(ctx) == nil.
				// When ctx is created using WithoutCancel.
				return nil
			}
		case *timerCtx:
			if key == &cancelCtxKey {
				return &ctx.cancelCtx
			}
		case backgroundCtx, todoCtx:
			return nil
		}
		c = c.Context
	}
}
```

# Best Practices

## Pass Context Explicitely

```go
// Wrong.
type s struct {
	ctx   context.Context
	field any
}

func (s s) f(args ...any) {
	// ...
}
```

```go
// Correct.
type s struct {
	field any
}

func (s s) f(ctx context.Context, args ...any) {
	// ...
}
```

## Use TODO instead of Nil

```go
func f(ctx context.Context) {
	// ...
}

// Wrong.
func caller() {
	f(nil)
}
```

```go
func f(ctx context.Context) {
	// ...
}

// Correct.
func caller() {
	f(context.TODO())
}

```

## Do Not Pass Optional Arguments via Context

```go
// Wrong.
func f(ctx context.Context) {
	optStr, ok := ctx.Value("some-key").(string)
	if !ok {
		// ...
	}
	// ...
}
```

```go
// Correct.
func f(ctx context.Context, optStr *string) {
	if optStr == nil {
		// ...
	}
	// ...
}
```

## Do Not Use Buint-In Types as Keys

```go
// Wrong.
func f(ctx context.Context) {
	metadata := ctx.Value("x-header")
    // ...
}
```

```go
// Correct.
type (
	firstKeyCorrect  struct{}
	secondKeyCorrect struct{}
)

func f(ctx context.Context) {
	firstValue := ctx.Value(firstKeyCorrect{})
	secondValue := ctx.Value(secondKeyCorrect{})
	// ...
}
```

```go
type (
	firstKeyCollision  = struct{}
	secondKeyCollision = struct{}
)

func f(ctx context.Context) {
	// Collision: firstValue == second value!
	firstValue := ctx.Value(firstKeyCollision{})
	secondValue := ctx.Value(secondKeyCollision{})
	// ...
}
```

## Be Sure to Cancel

```go
// Wrong.
func f(ctx context.Context) {
	ctx, _ = context.WithCancel(ctx)
	// ...
}
```

```go
// Correct.
func f(ctx context.Context) {
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)
	defer cancel()
	// ...
}
```

# Conclusion
