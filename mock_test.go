package vermock_test

import (
	"testing"

	vermock "github.com/Versent/go-vermock"
)

func TestNew_identity(t *testing.T) {
	t.Run("mockCache", func(t *testing.T) {
		m1 := vermock.New[mockCache](t)
		m2 := vermock.New[mockCache](t)
		if m1 == m2 {
			t.Error("expected different mocks")
		}
	})

	t.Run("vermock.Delegates", func(t *testing.T) {
		type T vermock.Delegates
		m1 := vermock.New[T](t)
		m2 := vermock.New[T](t)
		if m1 == m2 {
			t.Error("expected different mocks")
		}
	})

	t.Run("zero-sized", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic")
			} else if r != "vermock.New: zero-sized type used to construct more than one mock: *vermock_test.T" {
				t.Error("unexpected panic:", r)
			}
		}()
		type T struct{}
		_ = vermock.New[T](t)
		_ = vermock.New[T](t)
	})
}

func TestNew_Expect(t *testing.T) {
	called := false
	var cache Cache = vermock.New(&testing.T{},
		vermock.Expect[mockCache]("Put", func(_ testing.TB, key string, value any) error {
			if key != "foo" && value != "bar" {
				t.Error("unexpected arguments")
			}
			called = true
			return nil
		}),
		vermock.Expect[mockCache]("Get", func(_ *testing.T, key string) (any, bool) {
			if key != "foo" {
				t.Error("unexpected arguments")
			}
			called = true
			return "bar", true
		}),
		vermock.Expect[mockCache]("Delete", func(key string) {
			if key != "foo" {
				t.Error("unexpected arguments")
			}
			called = true
		}),
		ExpectDelete(func(_ testing.TB, key string) {
			t.Error("this should not be called")
		}),
	)

	called = false
	if err := cache.Put("foo", "bar"); err != nil {
		t.Error("unexpected error:", err)
	}
	if !called {
		t.Error("expected call to Put delegate")
	}

	called = false
	if result, ok := cache.Get("foo"); result != "bar" && ok {
		t.Error("unexpected result")
	}
	if !called {
		t.Error("expected call to Get delegate")
	}

	called = false
	cache.Delete("foo")
	if !called {
		t.Error("expected call to Delete delegate")
	}
}
