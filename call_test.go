package mock

import (
	"errors"
	"reflect"
	"testing"
)

func TestDoCall(t *testing.T) {
	tests := []struct {
		name        string
		callables   Callables
		in          []reflect.Value
		out         []reflect.Value
		results     []reflect.Value
		expectFail  bool
		expectPanic bool
	}{
		{
			name: "Matching types and values",
			callables: Callables{Value(reflect.ValueOf(func(t testing.TB, in string) string {
				if in != "input" {
					t.Errorf("unexpected input: expected %q, got %q", "input", in)
				}
				return "result"
			}))},
			in:         toValues("input"),
			out:        toValues(new(string)),
			results:    toValues("result"),
			expectFail: false,
		},
		{
			name: "Matching types and values, multi",
			callables: Callables{multi(reflect.ValueOf(func(t testing.TB, count CallCount, in string) string {
				if count != 0 {
					t.Errorf("unexpected count: expected %d, got %d", 0, count)
				}
				if in != "input" {
					t.Errorf("unexpected input: expected %q, got %q", "input", in)
				}
				return "result"
			}))},
			in:         toValues("input"),
			out:        toValues(new(string)),
			results:    toValues("result"),
			expectFail: false,
		},
		{
			name: "Matching types and values, variadic",
			callables: Callables{Value(reflect.ValueOf(func(t testing.TB, in ...string) string {
				if in[0] != "input" {
					t.Errorf("unexpected input: expected %q, got %q", "input", in)
				}
				return "result"
			}))},
			in:         toValues([]string{"input"}),
			out:        toValues(new(string)),
			results:    toValues("result"),
			expectFail: false,
		},
		{
			name: "Type mismatch",
			callables: Callables{Value(reflect.ValueOf(func() string {
				return "result"
			}))},
			in:          toValues(),
			out:         toValues(new(int)),
			results:     toValues(0),
			expectFail:  true,
			expectPanic: true,
		},
		{
			name:        "Unexpected number of results, panic",
			callables:   Callables{Value(reflect.ValueOf(func() {}))},
			in:          toValues(),
			out:         toValues(new(int)),
			results:     toValues(0),
			expectFail:  true,
			expectPanic: true,
		},
		{
			name:       "Unexpected number of results, error",
			callables:  Callables{Value(reflect.ValueOf(func() {}))},
			in:         toValues(),
			out:        toValues(new(error)),
			results:    toValues(errors.New("unexpected number of results: expected 1, got 0")),
			expectFail: true,
		},
		{
			name:       "Unexpected number of calls",
			callables:  Callables{},
			in:         toValues(),
			out:        toValues(new(error)),
			results:    toValues(errors.New("unexpected call to testMethod")),
			expectFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := &tt.name
			mockT := new(testing.T)
			defer func() {
				// Check for errors in test output
				if tt.expectFail && !mockT.Failed() {
					t.Errorf("expected a failure, got none")
				} else if !tt.expectFail && mockT.Failed() {
					t.Errorf("expected no failure, got fail")
				}
				if len(tt.out) != len(tt.results) {
					t.Fatalf("expected %d results, got %d", len(tt.results), len(tt.out))
				}
				for i := range tt.results {
					if !reflect.DeepEqual(tt.out[i].Elem().Interface(), tt.results[i].Interface()) {
						t.Errorf("out[%d]: expected %q, got %q", i, tt.results[i].Interface(), tt.out[i].Elem().Interface())
					}
				}
			}()
			if tt.expectPanic {
				defer func() {
					if recover() == nil {
						t.Errorf("Expected a panic, got none")
					}
				}()
			}
			registry[key] = &mock{
				TB: mockT,
				Delegates: Delegates{
					"testMethod": &Delegate{
						Callables: tt.callables,
					},
				},
			}
			t.Cleanup(func() {
				delete(registry, key)
			})

			// Call doCall
			doCall(key, "testMethod", tt.in, tt.out)
		})
	}
}
