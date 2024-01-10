package vermock_test

import (
	"fmt"
	"testing"

	vermock "github.com/Versent/go-vermock"
)

// Cache contains a variety of methods with different signatures.
type Cache interface {
	Put(string, any) error
	Get(string) (any, bool)
	Delete(string)
	Load(...string)
}

// mockCache is a mock implementation of Cache.  It can be anything, but
// zero-sized types are problematic.
type mockCache struct {
	_ byte // prevent zero-sized type
}

// Put returns one value, so use vermock.Call1.
func (m *mockCache) Put(key string, value any) error {
	return vermock.Call1[error](m, "Put", key, value)
}

// Get returns two values, so use vermock.Call2.
func (m *mockCache) Get(key string) (any, bool) {
	return vermock.Call2[any, bool](m, "Get", key)
}

// Delete returns no values, so use vermock.Call0.
func (m *mockCache) Delete(key string) {
	vermock.Call0(m, "Delete", key)
}

// Load is variadic, the last argument must be passed as a slice to one of the
// vermock.CallN functions.
func (m *mockCache) Load(keys ...string) {
	vermock.Call0(m, "Load", keys)
}

// UnusedCache is useful to show that a test's intent is that none of the
// interface methods are called.
var UnusedCache func(*mockCache) = nil

func ExampleUnusedCache() {
	t := &exampleT{} // or any testing.TB, your test does not create this
	// 1. Create a mock object.
	var cache Cache = vermock.New(t, UnusedCache)
	// 2. Use the mock object in your code under test.
	// 3. Assert that all expected methods were called.
	vermock.AssertExpectedCalls(t, cache)
	// mock will fail a test if a call is made to an unexpected method or if
	// the expected methods are not called.
	fmt.Println("less than expected:", t.Failed())
	// Output:
	// less than expected: false
}

// ExpectDelete is a helper function that hides the stringiness of vermock.
func ExpectDelete(delegate func(t testing.TB, key string)) func(*mockCache) {
	return vermock.Expect[mockCache]("Delete", delegate)
}

func Example_pass() {
	t := &exampleT{} // or any testing.TB, your test does not create this
	// 1. Create a mock object with expected calls.
	var cache Cache = vermock.New(t,
		// delegate function can receive testing.TB
		vermock.Expect[mockCache]("Get", func(t testing.TB, key string) (any, bool) {
			return "bar", true
		}),
		vermock.Expect[mockCache]("Put", func(t testing.TB, key string, value any) error {
			return nil
		}),
		// or only the method arguments
		vermock.Expect[mockCache]("Delete", func(key string) {}),
		// you may prefer to define a helper function
		ExpectDelete(func(t testing.TB, key string) {}),
	)
	// 2. Use the mock object in your code under test.
	cache.Put("foo", "bar")
	cache.Get("foo")
	cache.Delete("foo")
	cache.Delete("foo")
	// 3. Assert that all expected methods were called.
	vermock.AssertExpectedCalls(t, cache)
	// mock will not fail the test
	fmt.Println("less than expected:", t.Failed())
	// Output:
	// call to Put: 0/0
	// call to Get: 0/0
	// call to Delete: 0/0
	// call to Delete: 1/0
	// less than expected: false
}

func Example_unmetExpectation() {
	t := &testing.T{} // or any testing.TB, your test does not create this
	// 1. Create a mock object with expected calls.
	var cache Cache = vermock.New(t,
		// delegate function can receive testing.TB
		vermock.Expect[mockCache]("Put", func(t testing.TB, key string, value any) error {
			fmt.Println("put", key, value)
			return nil
		}),
		// or *testing.T
		vermock.Expect[mockCache]("Get", func(t *testing.T, key string) (any, bool) {
			fmt.Println("get", key)
			return "bar", true
		}),
		// or only the method arguments
		vermock.Expect[mockCache]("Delete", func(key string) {
			fmt.Println("delete", key)
		}),
		// you may prefer to define a helper function
		ExpectDelete(func(t testing.TB, key string) {
			t.Log("this is not going to be called; causing t.Fail() to be called by vermock.AssertExpectedCalls")
		}),
	)
	// 2. Use the mock object in your code under test.
	cache.Put("foo", "bar")
	cache.Get("foo")
	cache.Delete("foo")
	// 3. Assert that all expected methods were called.
	vermock.AssertExpectedCalls(t, cache)
	// mock will fail the test because the second call to Delete is not met.
	fmt.Println("less than expected:", t.Failed())
	// Output:
	// put foo bar
	// get foo
	// delete foo
	// less than expected: true
}

func Example_unexpectedCall() {
	t := &testing.T{} // or any testing.TB, your test does not create this
	// 1. Create a mock object with expected calls.
	var cache Cache = vermock.New(t,
		// delegate function can receive testing.TB
		vermock.Expect[mockCache]("Put", func(t testing.TB, key string, value any) error {
			fmt.Println("put", key, value)
			return nil
		}),
		// or only the method arguments
		vermock.Expect[mockCache]("Delete", func(key string) {
			fmt.Println("delete", key)
		}),
	)
	// 2. Use the mock object in your code under test.
	cache.Put("foo", "bar")
	cache.Get("foo")
	cache.Delete("foo")
	// 3. Assert that all expected methods were called.
	vermock.AssertExpectedCalls(t, cache)
	// mock will fail the test because the call to Get is not expected.
	fmt.Println("more than expected:", t.Failed())
	// Output:
	// put foo bar
	// delete foo
	// more than expected: true
}

func Example_allowRepeatedCalls() {
	t := &testing.T{} // or any testing.TB, your test does not create this
	// 1. Create a mock object with ExpectMany.
	var cache Cache = vermock.New(t,
		// delegate function may receive a call counter and the method arguments
		vermock.ExpectMany[mockCache]("Load", func(n vermock.CallCount, keys ...string) {
			fmt.Println("load", n, keys)
		}),
		// and testing.TB
		vermock.ExpectMany[mockCache]("Load", func(t testing.TB, n vermock.CallCount, keys ...string) {
			fmt.Println("load", n, keys)
		}),
		// or *testing.T
		vermock.ExpectMany[mockCache]("Load", func(t *testing.T, n vermock.CallCount, keys ...string) {
			fmt.Println("load", n, keys)
		}),
		// or only testing.TB/*testing.T
		vermock.ExpectMany[mockCache]("Load", func(t testing.TB, keys ...string) {
			fmt.Println("load 3", keys)
		}),
		// or only the method arguments
		vermock.ExpectMany[mockCache]("Load", func(keys ...string) {
			fmt.Println("load 4", keys)
		}),
	)
	// 2. Use the mock object in your code under test.
	cache.Load("foo", "bar")
	cache.Load("baz")
	cache.Load("foo")
	cache.Load("bar")
	cache.Load("baz")
	cache.Load("foo", "bar", "baz")
	// 3. Assert that all expected methods were called.
	vermock.AssertExpectedCalls(t, cache)
	// mock will not fail the test because ExpectMany allows repeated calls.
	fmt.Println("more than expected:", t.Failed())
	// Output:
	// load 0 [foo bar]
	// load 1 [baz]
	// load 2 [foo]
	// load 3 [bar]
	// load 4 [baz]
	// load 4 [foo bar baz]
	// more than expected: false
}

func Example_orderedCalls() {
	t := &testing.T{} // or any testing.TB, your test does not create this
	// 1. Create a mock object with ExpectInOrder.
	var cache Cache = vermock.New(t,
		vermock.ExpectInOrder(
			vermock.Expect[mockCache]("Put", func(key string, value any) error {
				fmt.Println("put", key, value)
				return nil
			}),
			vermock.Expect[mockCache]("Get", func(key string) (any, bool) {
				fmt.Println("get", key)
				return "bar", true
			}),
		),
	)
	// 2. Use the mock object in your code under test.
	cache.Get("foo")
	cache.Put("foo", "bar")
	// 3. Assert that all expected methods were called.
	vermock.AssertExpectedCalls(t, cache)
	// mock will fail the test because the call to Get is before the call
	// to Put.
	fmt.Println("less than expected:", t.Failed())
	// Output:
	// get foo
	// put foo bar
	// less than expected: true
}

func Example_mixedOrderedCalls() {
	t := &exampleT{} // or any testing.TB, your test does not create this
	// 1. Create a mock object with ExpectInOrder.
	get := vermock.Expect[mockCache]("Get", func(key string) (any, bool) {
		return "bar", true
	})
	put := vermock.Expect[mockCache]("Put", func(key string, value any) error {
		return nil
	})
	var cache Cache = vermock.New(t,
		get, put,
		vermock.ExpectInOrder(put, get),
		get, put,
	)
	// 2. Use the mock object in your code under test.
	for i := 0; i < 3; i++ {
		cache.Put(fmt.Sprint("foo", i), "bar")
		cache.Get(fmt.Sprint("foo", i))
	}
	// 3. Assert that all expected methods were called.
	vermock.AssertExpectedCalls(t, cache)
	// mock will not fail the test
	fmt.Println("less than expected:", t.Failed())
	// Output:
	// call to Put: 0/0
	// call to Get: 0/0
	// call to Put: 1/1
	// call to Get: 1/2
	// call to Put: 2/2
	// call to Get: 2/2
	// less than expected: false
}

var _ testing.TB = &exampleT{}

type exampleT struct {
	testing.T
}

func (t *exampleT) Fatal(args ...any) {
	fmt.Println(args...)
	t.T.FailNow()
}

func (t *exampleT) Fatalf(format string, args ...any) {
	fmt.Printf(format+"\n", args...)
	t.T.FailNow()
}

func (t *exampleT) Error(args ...any) {
	fmt.Println(args...)
	t.T.Fail()
}

func (t *exampleT) Errorf(format string, args ...any) {
	fmt.Printf(format+"\n", args...)
	t.T.Fail()
}

func (t *exampleT) Log(args ...any) {
	fmt.Println(args...)
}

func (t *exampleT) Logf(format string, args ...any) {
	fmt.Printf(format+"\n", args...)
}
