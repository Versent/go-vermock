# `mock` module

Tired of mocking libraries with cumbersome APIs?  Frustrated with numerous and complicated options?
Looking for a mock that works well with a composite of small interfaces or loves high ordered
functions?
Introducing `mock`, the simple mocking support that will enthusiastically accept a function that
can be tailored to any bespoke test case.
`mock` is guided by a central principle: test code must have full control of the code that runs in
the mocked object.  This means mock behaviour has access to anything in the test fixture and the
`testing.T` value.
This module provides a number functions that can be used as building blocks for your own mocks.

## Installation

To use the mock module, ensure it is installed and imported in your project.

```go
import mock "github.com/Versent/go-mock"
```

To use the mockgen command, simply run go run:

```sh
go run github.com/Versent/go-mock/cmd/mockgen
```

After running mockgen for the first time, go generate can be used to regenerate generated files:

```sh
go generate
```

## Basic Usage

1. **Define an Interface**

  Create one or more interfaces that your mock needs to satisfy.  For example:

  ```go
  package my

  type Getter interface {
  	Get(key string) (any, bool)
  }

  type Putter interface {
  	Put(key string, value any) error
  }
  ```

2. **Create a Mock Implementation**

  Implement the interface with mock methods. For example:

  ```go
  type mockObject struct {
  	_ byte // prevent zero-sized type
  }

  func (m *mockObject) Get(key string) (any, bool) {
  	return mock.Call2[any, bool](m, "Get", key)
  }

  func (m *mockObject) Put(key string, value any) error {
  	return mock.Call1[error](m, "Put", key, value)
  }
  ```

  Note that `mock.New` will panic if a zero-sized type is constructed more than once.

3. (Optional) **Define Helpers**

  Implement Expect functions for greater readability. For example:

  ```go
  func ExpectGet(delegate func(testing.TB, string) (any, bool)) func(*mockObject) {
  	return mock.Expect[mockObject]("Get", delegate)
  }

  func ExpectPut(delegate func(testing.TB, string, any) error) func(*mockObject) {
  	return mock.Expect[mockObject]("Put", delegate)
  }
  ```

4. **Using the Mock in Tests**

  Create a mock instance in your test and use it as needed. For instance:

  ```go
  func TestObject(t *testing.T) {
  	m := mock.New(t,
    	mock.ExpectGet(func(t testing.TB, key string) (any, bool) {
		// Define your mock's behaviour
	}),
    	mock.ExpectPut(func(t testing.TB, key string, value any) error {
		// Define your mock's behaviour
	}),
    )

  	// Use the mock instance in your test

  	// Assert that all expected methods were called
	mock.AssertExpectedCalls(t, m)
  }
  ```

### Using mockgen

Alternatively, creating a mock implementation and associated helpers can be automated with mockgen.
Instead of implementing all the methods of your mock, simply declare the interfaces you want your
mock to satisfy in an ordinary go file and let mockgen do the rest.

To continue the example from above this file would look like:

```go
//go:build mockstub

package my

type mockObject struct {
	Getter
	Putter
}
```

This is an ordinary go source file with a special build tag: mockstub.  After running mockgen (see
Installation above) a new file called `mock_gen.go` will be created with a new definition of
`mockObject` (the build tag ensures that these two definitions do not collide) containing all the
generated methods and functions.

## Beyond Basic Usage

Be sure to checkout the Examples in the tests.

### Expect Variants

In addition to the `mock.Expect` function, which corresponds to a single call of a method,
there is also `mock.ExpectMany`, which will consume all remaining calls of a method.

Expect functions accepts a delegate function that matches the signature of the named method.
The delegate may also accept a `*testingT` or `testing.TB` value as the first argument.
This the same `testing.T` that was used to construct the mock (first argument to `mock.New`).
In addition, ExpectMany optionally accepts the method's call count.

### Ordered Calls

The `mock.ExpectInOrder` will ensure that calls occur in a specified order.
For example, this will fail if `Put` is called before `Get`:

```go
mock.New(t, mock.ExpectInOrder(mock.Expect("Get", ...), mock.Expect("Put", ...)))
```
