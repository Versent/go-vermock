package mock_test

import (
	"fmt"
	"testing"

	mock "gist.github.com/1bc0b2dc0e9680251778d289e5493826"
)

type Cache interface {
	Get(string) (any, bool)
	Put(string, any) error
	Delete(string)
}

type mockCache struct {
	_ byte // prevent zero-sized type
}

func (m *mockCache) Get(key string) (any, bool) {
	return mock.Call2[any, bool](m, "Get", key)
}

func (m *mockCache) Put(key string, value any) error {
	return mock.Call1[error](m, "Put", key, value)
}

func (m *mockCache) Delete(key string) {
	mock.Call0(m, "Delete", key)
}

var UnusedCache func(*mockCache) = nil

func ExampleUnusedCache() {
	t := &testing.T{} // or any testing.TB
	var cache Cache = mock.New(t, UnusedCache)
	mock.AssertExpectedCalls(t, cache)
	fmt.Println("less than expected:", t.Failed())
	// Output:
	// less than expected: false
}

func ExpectDelete(delegate func(t testing.TB, key string)) func(*mockCache) {
	return mock.Expect[mockCache]("Delete", delegate)
}

func Example() {
	t := &testing.T{} // or any testing.TB
	var cache Cache = mock.New(t,
		mock.Expect[mockCache]("Put", func(t testing.TB, key string, value any) error {
			fmt.Println("put", key, value)
			return nil
		}),
		mock.Expect[mockCache]("Get", func(t *testing.T, key string) (any, bool) {
			fmt.Println("get", key)
			return "bar", true
		}),
		mock.Expect[mockCache]("Delete", func(key string) {
			fmt.Println("delete", key)
		}),
		ExpectDelete(func(t testing.TB, key string) {
			t.Log("this should not be called; causing t.Fail() to be called by mock.AssertExpectedCalls")
		}),
	)
	cache.Put("foo", "bar")
	cache.Get("foo")
	cache.Delete("foo")
	mock.AssertExpectedCalls(t, cache)
	fmt.Println("less than expected:", t.Failed())
	// Output:
	// put foo bar
	// get foo
	// delete foo
	// less than expected: true
}
