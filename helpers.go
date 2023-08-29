package mock

import "testing"

func AssertExpectedCalls(t testing.TB, mocks ...any) {
	t.Helper()

	for _, key := range mocks {
		if key == nil {
			continue
		}

		mock, ok := registry[key]
		if !ok {
			t.Fatalf("mock not found: %T", key)
		}

		for name, delegate := range mock.Delegates {
			if count := delegate.callCount; count < delegate.Len() {
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

func Call0[T any](key *T, name string, in ...any) {
	registry[key].Helper()
	CallDelegate(key, name, nil, toValues(in...)...)
}

func Call1[T1, T any](key *T, name string, in ...any) (v T1) {
	registry[key].Helper()
	doCall(key, name, toValues(in...), toValues(&v))
	return
}

func Call2[T1, T2, T any](key *T, name string, in ...any) (v1 T1, v2 T2) {
	registry[key].Helper()
	doCall(key, name, toValues(in...), toValues(&v1, &v2))
	return
}

func Call3[T1, T2, T3, T any](key *T, name string, in ...any) (v1 T1, v2 T2, v3 T3) {
	registry[key].Helper()
	doCall(key, name, toValues(in...), toValues(&v1, &v2, &v3))
	return
}

func Call4[T1, T2, T3, T4, T any](key *T, name string, in ...any) (v1 T1, v2 T2, v3 T3, v4 T4) {
	registry[key].Helper()
	doCall(key, name, toValues(in...), toValues(&v1, &v2, &v3, &v4))
	return
}

func Call5[T1, T2, T3, T4, T5, T any](key *T, name string, in ...any) (v1 T1, v2 T2, v3 T3, v4 T4, v5 T5) {
	registry[key].Helper()
	doCall(key, name, toValues(in...), toValues(&v1, &v2, &v3, &v4, &v5))
	return
}

func Call6[T1, T2, T3, T4, T5, T6, T any](key *T, name string, in ...any) (v1 T1, v2 T2, v3 T3, v4 T4, v5 T5, v6 T6) {
	registry[key].Helper()
	doCall(key, name, toValues(in...), toValues(&v1, &v2, &v3, &v4, &v5, &v6))
	return
}

func Call7[T1, T2, T3, T4, T5, T6, T7, T any](key *T, name string, in ...any) (v1 T1, v2 T2, v3 T3, v4 T4, v5 T5, v6 T6, v7 T7) {
	registry[key].Helper()
	doCall(key, name, toValues(in...), toValues(&v1, &v2, &v3, &v4, &v5, &v6, &v7))
	return
}

func Call8[T1, T2, T3, T4, T5, T6, T7, T8, T any](key *T, name string, in ...any) (v1 T1, v2 T2, v3 T3, v4 T4, v5 T5, v6 T6, v7 T7, v8 T8) {
	registry[key].Helper()
	doCall(key, name, toValues(in...), toValues(&v1, &v2, &v3, &v4, &v5, &v6, &v7, &v8))
	return
}

func Call9[T1, T2, T3, T4, T5, T6, T7, T8, T9, T any](key *T, name string, in ...any) (v1 T1, v2 T2, v3 T3, v4 T4, v5 T5, v6 T6, v7 T7, v8 T8, v9 T9) {
	registry[key].Helper()
	doCall(key, name, toValues(in...), toValues(&v1, &v2, &v3, &v4, &v5, &v6, &v7, &v8, &v9))
	return
}
