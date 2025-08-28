# Функциональность

- Получение дэдлайна
- Синхронизация с помощью каналов
- Получение причины завершения
- KeyValue-хранилище

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

# История

Контекст, на самом деле, появился далеко не сразу

## Раньше

Раньше для синхронизации использовались каналы, однако это было не так удобно

Тем не менее, в примере ниже легко узнать `context.WithTimeout()`

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

## Сейчас

В 2014 появился пакет golang.org/net/context, ставший крайне популярным, особенно среди инженеров из Google

Поэтому через 2 года его затащили в STL на версии Go 1.7

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

# Виды и их реализация

## Empty Context

Очень простая реализация:
- Дэдлайна нет
- Канал пустой, так как навечно блокирует, что ожидаемо, так как контекст никогда не завершится
- Причина всегда пустая, так как её нет
- Значения также не храним

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

- `context.TODO()` - плейсхолдер
- `context.Background()` - корневая нода для всех остальных контекстов

## Cancel Context

Включает:
- Родительский контекст
- Канал, который хранится в атомике
- Набор потомков, которых можно отменить
- Информацию об отмене

Все поля обёрнуты мьютексом, но для простоты не будем на нём заострять внимание

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

Ещё есть 2 магических значения:
- Канал, который закрывается при инициализации пакета
- Переменная типа int с дефолтным значением

Зачем они нужны - узнаем далее

```go
var closedchan = make(chan struct{})

func init() {
	close(closedchan)
}

var cancelCtxKey int
```

Рассмотрим реализацию методов:
- `Deadline` возвращается родительский, так как текущий контекст его не менял
- `Done` инициализируется лениво
- `Err` возвращает текущее значение поля `err`
- `Value` возвращает сам контекст в случае, когда магическое значение адреса переменной `cancelCtxKey`, нужно это только для внутренней реализации, в другом же случае ищем значение у родителя

Такой необычный ключ для `Value` сделан для того, чтобы его тяжело было предугадать. Дело в том, что переменная выделяется на heap, а указатель на неё - это адрес в памяти. Фактически, это случайное целое число

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

Создание происходит в 2 основных этапа:
- "Распространение" отмены на потомков
- Создание контекста и ручки отмены

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

Распространение состоит из трёх основных этапов:
- Если родительский контекст - это `context.WithTimeout()`, делаем текущий зависимым от него. Если он уже был отменён - отменяем и текущий
- В случае `AfterFunc` "склеиваем" отмену через его хэндлер, более подробно обсудим позже
- В отдельной горутине ожидаем окончание родителя и потомка. Если раньше заверился первый, то отменяем оба

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

При отмене:
- Проверяем, не был ли контекст уже отменён
- Присваиваем причину
- Закрываем канал done (помним о ленивой инициализации, поэтому если `ctx.Done()` ни разу не использовался - кладём туда магический `closedchan`)
- Отменяем всех потомков
- Флаг `removeFromParent` показывает, нужно ли удалять потомков или нет, так как иногда они могут быть ещё не добавлены или несколько раз каскадно удаляться

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

Соответственно, используем данный контекст для ручной отмены

Например, если пользователь отменил запрос

## Context without Cancel

`context.WithoutCancel()` "забывает" о том, что ему надо отмениться

Соответственно:
- Дэдлайна нет
- Канал пустой для вечной блокировки
- Причины отмены нет
- Значение же получаем у родителя

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

Нужен такой контекст в случаях, когда мы хотим сохранить метаданные, но не хотим отменяться

Например, при походе в сервис 

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
