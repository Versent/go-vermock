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
import mock "gist.github.com/1bc0b2dc0e9680251778d289e5493826"
```

## Basic Usage

1. **Define an Interface**

  Create one or more interfaces that your mock needs to satisfy.  For example:

  ```go
  package my

  type Getter interface {
  	Get(string) (any, bool)
  }

  type Putter interface {
  	Put(string, any) error
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
    	mock.ExpectGet(func(t testing.TB, key string) (any, bool) {...}),
    	mock.ExpectPut(func(t testing.TB, key string, value any) error {...}),
    )

  	// Use the mock instance in your test

  	// Assert that all expected methods were called
	mock.AssertExpectedCalls(t, m)
  }
  ```

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
