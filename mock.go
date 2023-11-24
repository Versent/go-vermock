// Package mock provides a flexible and functional mocking framework for Go
// tests.
package mock

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
)

var (
	// registry holds the active mock objects.
	registry = make(map[any]*mock)
)

// Delegates maps function names to their Delegate implementations.
type Delegates = map[string]*Delegate

// Option defines a function that configures a mock object.
type Option[T any] func(*T)

func Options[T any](opts ...Option[T]) Option[T] {
	return func(key *T) {
		for _, opt := range opts {
			opt(key)
		}
	}
}

// mock represents a mock object.
type mock struct {
	testing.TB
	sync.Mutex
	Delegates
	ordered
}

// New creates a new mock object of type T and applies the given options.
// It panics if a mock for a zero-sized type is constructed more than once.
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
	mock.ordinal = 0
	return key
}

// Expect registers a function to be called exactly once when a method with the
// given name is invoked on the mock object.
// The function signature of fn must match the named method signature,
// except that the first argument may optionally be a testing.TB or *testing.T.
// Panics if fn is not a function.
func Expect[T any](name string, fn any) Option[T] {
	funcType := reflect.TypeOf(fn)
	if funcType.Kind() != reflect.Func {
		panic(fmt.Sprintf("mock.Expect: expected function, got %T", fn))
	}
	return func(key *T) {
		mock := registry[key]
		mock.Helper()
		delegate := delegateByName(mock, name)
		if mock.inOrder {
			mock.ordinal++
		}
		delegate.Append(Value{
			Value:   reflect.ValueOf(fn),
			ordered: mock.ordered,
		})
	}
}

// ExpectMany registers a function to be called at least once for a method with
// the given name on the mock object.
// Like Expect, the arguments of fn must match the named method signature and may optionally be
// preceded by a testing.TB or *testing.T.
// In addition, the first argument of fn may optionally be of type CallCount, in such cases fn will
// be passed the total number of times the method has been called (starting at 0).
// Panics if fn is not a function.
func ExpectMany[T any](name string, fn any) Option[T] {
	funcType := reflect.TypeOf(fn)
	if funcType.Kind() != reflect.Func {
		panic(fmt.Sprintf("mock.ExpectMany: expected function, got %T", fn))
	}
	return func(key *T) {
		mock := registry[key]
		mock.Helper()
		if mock.inOrder {
			mock.ordinal++
		}
		delegateByName(mock, name).Append(multi{
			Value:   reflect.ValueOf(fn),
			ordered: mock.ordered,
		})
	}
}
