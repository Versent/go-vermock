package mock

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
)

var (
	registry = make(map[any]*mock)
)

type Delegates = map[string]*Delegate

type Option[T any] func(*T)

type mock struct {
	testing.TB
	sync.Mutex
	Delegates
}

func New[T any](t testing.TB, opts ...Option[T]) *T {
	key := new(T)
	mock := &mock{
		TB:        t,
		Delegates: Delegates{},
	}
	if _, ok := registry[key]; ok {
		panic(fmt.Sprintf("mock.New: zero-sized type used to construct more than one mock: %T", key))
	}
	registry[key] = mock
	t.Cleanup(func() {
		delete(registry, key)
	})
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(key)
	}
	return key
}

func Expect[T any](name string, fn any) Option[T] {
	return func(key *T) {
		mock := registry[key]
		mock.Helper()
		delegateByName(mock, name).Append(Value(reflect.ValueOf(fn)))
	}
}

func ExpectMany[T any](name string, fn any) Option[T] {
	return func(key *T) {
		mock := registry[key]
		mock.Helper()
		delegateByName(mock, name).Append(multi(reflect.ValueOf(fn)))
	}
}
