package chans

type Chan[T any] <-chan T

func (c Chan[T]) Next() (T, bool) {
	t, ok := <-c
	return t, ok
}

type Sink[T any] chan<- T

type Functor[T, R any] func(Chan[T]) Chan[R]

type Bifunctor[T, U, R any] func(Chan[T], Chan[U]) Chan[R]

func Map[T, R any](f func(T) R) Functor[T, R] {
	return newFunctor(func(in Chan[T], out Sink[R]) {
		for v := range in {
			out <- f(v)
		}
	})
}

func BatchFold[T, A any](f func(A, T) A) Functor[T, A] {
	return newFunctor(func(in Chan[T], out Sink[A]) {
		for t := range in {
			var batch A
			batch = f(batch, t)
		Send:
			for {
				select {
				case t, ok := <-in:
					if !ok {
						return
					}
					batch = f(batch, t)
				case out <- batch:
					break Send
				}
			}
		}
	})
}

func Batch[T any](in Chan[T]) Chan[[]T] {
	return BatchFold(func(acc []T, t T) []T {
		return append(acc, t)
	})(in)
}

func MapSidechain[T, U, R any](f func(T, U) R) Bifunctor[T, U, R] {
	return newBifunctor(func(signal Chan[T], peripheral Chan[U], out Sink[R]) {
		for {
			side := peripheral
			var (
				sig T
				acc U
			)
		WaitForSignal:
			select {
			case t, ok := <-signal:
				if !ok {
					return
				}
				sig = t
			case u, ok := <-side:
				if !ok {
					peripheral = nil
				} else {
					acc = u
				}
				side = nil
				goto WaitForSignal
			}
		WaitForSend:
			select {
			case out <- f(sig, acc):
			case u, ok := <-side:
				if !ok {
					peripheral = nil
				} else {
					acc = u
				}
				side = nil
				goto WaitForSend
			}
		}
	})
}

func Sidechain[T, U any](signal Chan[T], peripheral Chan[U]) Chan[Pair[T, U]] {
	return MapSidechain(PairOf[T, U])(signal, peripheral)
}

type Pair[L, R any] struct {
	L L
	R R
}

func PairOf[L, R any](left L, right R) Pair[L, R] {
	return Pair[L, R]{L: left, R: right}
}

func Pipe[T, R any](in Chan[T], f Functor[T, R]) Chan[R] {
	return f(in)
}

func Compose[T, U, R any](f Functor[T, U], g Functor[U, R]) Functor[T, R] {
	return func(in Chan[T]) Chan[R] {
		return g(f(in))
	}
}

func newFunctor[T, R any](f func(in Chan[T], out Sink[R])) Functor[T, R] {
	return func(in Chan[T]) Chan[R] {
		out := make(chan R)
		go func() {
			defer close(out)
			f(in, out)
		}()
		return out
	}
}

func newBifunctor[T, U, R any](f func(in1 Chan[T], in2 Chan[U], out Sink[R])) Bifunctor[T, U, R] {
	return func(in1 Chan[T], in2 Chan[U]) Chan[R] {
		out := make(chan R)
		go func() {
			defer close(out)
			f(in1, in2, out)
		}()
		return out
	}
}
