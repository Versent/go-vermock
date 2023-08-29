package mock

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

type Callable interface {
	Call(testing.TB, int, []reflect.Value) []reflect.Value
}

type MultiCallable interface {
	MultiCallable() bool
}

type Callables []Callable

func (c Callables) Len() int {
	return len(c)
}

func (c Callables) Cap() int {
	return cap(c)
}

func (c Callables) Append(callable ...Callable) Callables {
	return append(c, callable...)
}

func (c Callables) Call(t testing.TB, i int, in []reflect.Value) []reflect.Value {
	return c[i].Call(t, i, in)
}

func (c Callables) MultiCallable() bool {
	if len(c) == 0 {
		return false
	}
	if m, ok := c[len(c)-1].(MultiCallable); ok {
		return m.MultiCallable()
	}
	return false
}

type Value reflect.Value

func (v Value) Call(t testing.TB, count int, in []reflect.Value) []reflect.Value {
	fn := reflect.Value(v)
	if fn.Kind() != reflect.Func {
		panic(fmt.Sprintf("expected func, got %T", v))
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

type multi Value

func (v multi) MultiCallable() {}

func (v multi) Call(t testing.TB, count int, in []reflect.Value) []reflect.Value {
	in = append([]reflect.Value{reflect.ValueOf(count)}, in...)
	return Value(v).Call(t, count, in)
}

var errType = reflect.TypeOf((*error)(nil)).Elem()

func CallDelegate[T any](key *T, name string, outTypes []reflect.Type, in ...reflect.Value) (out []reflect.Value) {
	mock := registry[key]
	t := mock.TB
	t.Helper()

	delegate := delegateByName(mock, name)
	delegate.Lock()
	defer delegate.Unlock()

	var fn Callable
	if delegate.callCount < delegate.Len() {
		fn = delegate
	} else if !delegate.MultiCallable() {
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
	t.Logf("call to %s: %d", name, delegate.callCount)
	defer func() { delegate.callCount++ }()
	return fn.Call(t, delegate.callCount, in)
}

func multiCallable(funcs Callables) (fn Callable, ok bool) {
	if len(funcs) == 0 {
		return
	}
	fn = funcs[len(funcs)-1]
	_, ok = fn.(MultiCallable)
	return
}

func toValues(in ...any) (out []reflect.Value) {
	out = make([]reflect.Value, len(in))
	for i, v := range in {
		out[i] = reflect.ValueOf(v)
	}
	return
}

func doCall[T any](key *T, name string, in []reflect.Value, out []reflect.Value) {
	registry[key].Helper()
	outTypes := make([]reflect.Type, len(out))
	for i := range out {
		outTypes[i] = out[i].Type().Elem()
	}
	results := CallDelegate(key, name, outTypes, in...)
	last := len(outTypes) - 1
	for i := range out {
		var err error
		if !results[i].IsZero() {
			if results[i].Type().AssignableTo(outTypes[i]) {
				out[i].Elem().Set(results[i])
			} else {
				err = fmt.Errorf("unexpected type %T for result parameter %T", results[i].Interface(), out[i].Interface())
			}
		}
		if err != nil {
			t2 := outTypes[last]
			if reflect.TypeOf(err).ConvertibleTo(t2) {
				out[last].Elem().Set(reflect.ValueOf(err).Convert(t2))
				return
			} else {
				panic(err)
			}
		}
	}
}
