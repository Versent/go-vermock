package vermock

import "testing"

// AssertExpectedCalls asserts that all expected callables of all delegates of
// the given mocks were called.
func AssertExpectedCalls(t testing.TB, mocks ...any) {
	t.Helper()

	for _, key := range mocks {
		if key == nil {
			continue
		}

		if mock, ok := key.(interface{ AssertExpectedCalls(testing.TB) }); ok {
			mock.AssertExpectedCalls(t)
			continue
		}

		mock, ok := registry[key]
		if !ok {
			t.Fatalf("mock not found: %T", key)
		}

		for name, delegate := range mock.Delegates {
			if count := delegate.callCount; int(count) < delegate.Len() {
				if count == 0 {
					t.Errorf("failed to make call to %s", name)
				} else if count == 1 {
					t.Errorf("failed to make call to %s: only got one call", name)
				} else {
					t.Errorf("failed to make call to %s: only got %d calls", name, count)
				}
			}
		}
	}
}

// Call0 calls the function of the given name for the given mock with the
// given arguments.  If the function is variadic then the last argument must be
// passed as a slice, otherwise this function panics.  The function is expected
// to return no result values, otherwise the will be marked as a fail and this
// function will panic.
func Call0[T any](key *T, name string, in ...any) {
	registry[key].Helper()
	CallDelegate(key, name, nil, toValues(in...)...)
}

// Call1 calls the function of the given name for the given mock with the
// given arguments.  If the function is variadic then the last argument must be
// passed as a slice, otherwise this function panics.  The function is expected
// to return one result value, otherwise the will be marked as a fail and this
// function will return an error when T1 is assignable to an error type, or
// this function will panic.
func Call1[T1, T any](key *T, name string, in ...any) (v T1) {
	registry[key].Helper()
	doCall(key, name, toValues(in...), toValues(&v))
	return
}

// Call2 calls the function of the given name for the given mock with the
// given arguments.  If the function is variadic then the last argument must be
// passed as a slice, otherwise this function panics.  The function is expected
// to return two result values, otherwise the will be marked as a fail and this
// function will return an error when T2 is assignable to an error type, or
// this function will panic.
func Call2[T1, T2, T any](key *T, name string, in ...any) (v1 T1, v2 T2) {
	registry[key].Helper()
	doCall(key, name, toValues(in...), toValues(&v1, &v2))
	return
}

// Call3 calls the function of the given name for the given mock with the
// given arguments.  If the function is variadic then the last argument must be
// passed as a slice, otherwise this function panics.  The function is expected
// to return three result values, otherwise the will be marked as a fail and
// this function will return an error when T3 is assignable to an error type,
// or this function will panic.
func Call3[T1, T2, T3, T any](key *T, name string, in ...any) (v1 T1, v2 T2, v3 T3) {
	registry[key].Helper()
	doCall(key, name, toValues(in...), toValues(&v1, &v2, &v3))
	return
}

// Call4 calls the function of the given name for the given mock with the
// given arguments.  If the function is variadic then the last argument must be
// passed as a slice, otherwise this function panics.  The function is expected
// to return four result values, otherwise the will be marked as a fail and
// this function will return an error when T4 is assignable to an error type,
// or this function will panic.
func Call4[T1, T2, T3, T4, T any](key *T, name string, in ...any) (v1 T1, v2 T2, v3 T3, v4 T4) {
	registry[key].Helper()
	doCall(key, name, toValues(in...), toValues(&v1, &v2, &v3, &v4))
	return
}

// Call5 calls the function of the given name for the given mock with the
// given arguments.  If the function is variadic then the last argument must be
// passed as a slice, otherwise this function panics.  The function is expected
// to return 5 result values, otherwise the will be marked as a fail and this
// function will return an error when T5 is assignable to an error type, or
// this function will panic.
func Call5[T1, T2, T3, T4, T5, T any](key *T, name string, in ...any) (v1 T1, v2 T2, v3 T3, v4 T4, v5 T5) {
	registry[key].Helper()
	doCall(key, name, toValues(in...), toValues(&v1, &v2, &v3, &v4, &v5))
	return
}

// Call6 calls the function of the given name for the given mock with the
// given arguments.  If the function is variadic then the last argument must be
// passed as a slice, otherwise this function panics.  The function is expected
// to return 6 result values, otherwise the will be marked as a fail and this
// function will return an error when T6 is assignable to an error type, or
// this function will panic.
func Call6[T1, T2, T3, T4, T5, T6, T any](key *T, name string, in ...any) (v1 T1, v2 T2, v3 T3, v4 T4, v5 T5, v6 T6) {
	registry[key].Helper()
	doCall(key, name, toValues(in...), toValues(&v1, &v2, &v3, &v4, &v5, &v6))
	return
}

// Call7 calls the function of the given name for the given mock with the
// given arguments.  If the function is variadic then the last argument must be
// passed as a slice, otherwise this function panics.  The function is expected
// to return 7 result values, otherwise the will be marked as a fail and this
// function will return an error when T7 is assignable to an error type, or
// this function will panic.
func Call7[T1, T2, T3, T4, T5, T6, T7, T any](key *T, name string, in ...any) (v1 T1, v2 T2, v3 T3, v4 T4, v5 T5, v6 T6, v7 T7) {
	registry[key].Helper()
	doCall(key, name, toValues(in...), toValues(&v1, &v2, &v3, &v4, &v5, &v6, &v7))
	return
}

// Call8 calls the function of the given name for the given mock with the
// given arguments.  If the function is variadic then the last argument must be
// passed as a slice, otherwise this function panics.  The function is expected
// to return 8 result values, otherwise the will be marked as a fail and this
// function will return an error when T8 is assignable to an error type, or
// this function will panic.
func Call8[T1, T2, T3, T4, T5, T6, T7, T8, T any](key *T, name string, in ...any) (v1 T1, v2 T2, v3 T3, v4 T4, v5 T5, v6 T6, v7 T7, v8 T8) {
	registry[key].Helper()
	doCall(key, name, toValues(in...), toValues(&v1, &v2, &v3, &v4, &v5, &v6, &v7, &v8))
	return
}

// Call9 calls the function of the given name for the given mock with the
// given arguments.  If the function is variadic then the last argument must be
// passed as a slice, otherwise this function panics.  The function is expected
// to return 9 result values, otherwise the will be marked as a fail and this
// function will return an error when T9 is assignable to an error type, or
// this function will panic.
func Call9[T1, T2, T3, T4, T5, T6, T7, T8, T9, T any](key *T, name string, in ...any) (v1 T1, v2 T2, v3 T3, v4 T4, v5 T5, v6 T6, v7 T7, v8 T8, v9 T9) {
	registry[key].Helper()
	doCall(key, name, toValues(in...), toValues(&v1, &v2, &v3, &v4, &v5, &v6, &v7, &v8, &v9))
	return
}
