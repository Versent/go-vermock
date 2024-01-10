package vermock

import (
	"testing"
)

func TestNew(t *testing.T) {
	type T Delegates
	mock := New[T](t)
	_, ok := registry[mock]
	if !ok {
		t.Fatalf("mock not found")
	}
}

func TestExpect(t *testing.T) {
	type T Delegates
	key := New(t, Expect[T]("foo", func() {}))

	mock, ok := registry[key]
	if !ok {
		t.Fatalf("mock not found")
	}

	fn, ok := mock.Delegates["foo"]
	if !ok {
		t.Fatalf("delegate not found")
	}

	if fn.Len() != 1 {
		t.Fatalf("expected one delegate")
	}
}
