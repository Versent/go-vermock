package vermock

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

// Callable defines an interface for delegates to call test functions.
type Callable interface {
	Call(testing.TB, CallCount, []reflect.Value) []reflect.Value
}

// MultiCallable defines an interface for Callable objects that can be called
// multiple times.
type MultiCallable interface {
	MultiCallable() bool
}

// Callables is a slice of Callable objects.
type Callables []Callable

// Len returns the number of Callables in the slice.
func (c Callables) Len() int {
	return len(c)
}

// Cap returns the capacity of the slice of Callables.
func (c Callables) Cap() int {
	return cap(c)
}

// Append adds one or more Callables to the slice.
func (c Callables) Append(callable ...Callable) Callables {
	return append(c, callable...)
}

// Call invokes the Callable at the given index with the given arguments.
// Panics if the index is out of range and the last Callable is not a
// MultiCallable.
func (c Callables) Call(t testing.TB, index CallCount, in []reflect.Value) []reflect.Value {
	if int(index) < len(c) {
		return c[index].Call(t, index, in)
	}
	if c.MultiCallable() {
		return c[len(c)-1].Call(t, index, in)
	}
	panic(fmt.Sprintf("Callables.Call: index out of range [%d] with length %d", index, len(c)))
}

// MultiCallable returns true if the last Callable in the slice is a
// MultiCallable.
func (c Callables) MultiCallable() bool {
	if len(c) == 0 {
		return false
	}
	if m, ok := c[len(c)-1].(MultiCallable); ok {
		return m.MultiCallable()
	}
	return false
}

// Value is a Callable that wraps a reflect.Value.
type Value struct {
	reflect.Value
	ordered
}

// Call invokes the Callable with the given arguments.  If the Callable is variadic,
// the last argument must be passed as a slice, otherwise this method panics.
func (v Value) Call(t testing.TB, i CallCount, in []reflect.Value) []reflect.Value {
	fn := v.Value
	if fn.Kind() != reflect.Func {
		panic(fmt.Sprintf("Value.Call: expected func, got %T", v))
	}
	if fn.Type().NumIn() == len(in)+1 {
		in = append([]reflect.Value{reflect.ValueOf(t)}, in...)
	}
	if fn.Type().IsVariadic() {
		return fn.CallSlice(in)
	} else {
		return fn.Call(in)
	}
}

// multi is a Callable that wraps a reflect.Value and implements MultiCallable.
type multi Value

// MultiCallable returns true.
func (v multi) MultiCallable() bool { return true }

// Call invokes the Callable with the given arguments.
func (v multi) Call(t testing.TB, i CallCount, in []reflect.Value) []reflect.Value {
	funcType := v.Value.Type()
	if funcType.NumIn() > 0 && funcType.In(0) == reflect.TypeOf(i) ||
		funcType.NumIn() > 1 && funcType.In(1) == reflect.TypeOf(i) {
		in = append([]reflect.Value{reflect.ValueOf(i)}, in...)
	}
	return Value(v).Call(t, i, in)
}

// errType is the type of the error interface.
var errType = reflect.TypeOf((*error)(nil)).Elem()

// CallDelegate calls the next Callable of the Delegate with the given name and
// given arguments.  If the delegate is variadic then the last argument must be
// a slice, otherwise this function panics.  If the next Callable does not
// exist or the last Callable is not MultiCallable, then the mock object will
// be marked as failed.  In the case of a fail and if the delegate function
// returns an error as its last return value, then the error will be set and
// returned otherwise the function returns zero values for all of the return
// values.
func CallDelegate[T any](key *T, name string, outTypes []reflect.Type, in ...reflect.Value) (out []reflect.Value) {
	mock := registry[key]
	t := mock.TB
	t.Helper()

	delegate := delegateByName(mock, name)
	delegate.Lock()
	defer delegate.Unlock()

	if int(delegate.callCount) >= delegate.Len() && !delegate.MultiCallable() {
		msg := "unexpected call to " + name
		t.Error(msg)
		out = make([]reflect.Value, 0, len(outTypes))
		for _, typ := range outTypes {
			out = append(out, reflect.Zero(typ))
		}
		// set last out to error
		if i := len(out) - 1; i >= 0 && outTypes[i].Implements(errType) {
			out[i] = reflect.ValueOf(errors.New(msg))
		}
		return
	}

	var (
		fn Value
		ok bool
	)
	if int(delegate.callCount) < delegate.Len() {
		fn, ok = delegate.Callables[delegate.callCount].(Value)
	} else {
		fn, ok = delegate.Callables[delegate.Len()-1].(Value)
	}

	if fn.inOrder {
		mock.ordinal++
	}

	if ok && fn.ordinal != mock.ordinal {
		err := fmt.Sprintf("out of order call to %s: expected %d, got %d", name, fn.ordinal, mock.ordinal)
		t.Error(err)
	}

	t.Logf("call to %s: %d/%d", name, delegate.callCount, mock.ordinal)
	defer func() { delegate.callCount++ }()
	return delegate.Call(t, delegate.callCount, in)
}

// toValues converts the given values to reflect.Values.
func toValues(in ...any) (out []reflect.Value) {
	out = make([]reflect.Value, len(in))
	for i, v := range in {
		out[i] = reflect.ValueOf(v)
	}
	return
}

// doCall calls the next Callable of the Delegate with the given name and given
// arguments and sets the given out values to the return values of the Callable.
// If the types of the return values do not match the types of the out values,
// or if the number of return values does not match the number of out values,
// then the last out value will be set to an error if it is assignable to an
// error type otherwise this function will panic.
func doCall[T any](key *T, name string, in []reflect.Value, out []reflect.Value) {
	registry[key].Helper()
	outTypes := make([]reflect.Type, len(out))
	for i := range out {
		outTypes[i] = out[i].Type().Elem()
	}
	results := CallDelegate(key, name, outTypes, in...)
	last := len(outTypes) - 1
	var err error
	if len(results) != len(outTypes) {
		err = fmt.Errorf("unexpected number of results: expected %d, got %d", len(outTypes), len(results))
	}
	for i := range out {
		if err != nil {
			break
		}
		if !results[i].IsZero() {
			if results[i].Type().AssignableTo(outTypes[i]) {
				out[i].Elem().Set(results[i])
			} else {
				err = fmt.Errorf("unexpected type %T for result parameter %T", results[i].Interface(), out[i].Interface())
			}
		}
	}
	if err != nil {
		registry[key].Error(err)
		t2 := outTypes[last]
		if reflect.TypeOf(err).ConvertibleTo(t2) {
			out[last].Elem().Set(reflect.ValueOf(err).Convert(t2))
		} else {
			panic(err)
		}
	}
}
