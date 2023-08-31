package mock

import "sync"

type CallCount int

// Delegate represents a function that is expected to be called.
type Delegate struct {
	sync.Mutex
	Callables
	callCount CallCount
}

// Append adds one or more callables to the delegate.
func (d *Delegate) Append(callable ...Callable) Callables {
	d.Lock()
	defer d.Unlock()
	d.Callables = d.Callables.Append(callable...)
	return d.Callables
}

// delegateByName retrieves or creates a Delegate for a given method name.  It
// is safe to call from multiple goroutines.
func delegateByName(mock *mock, name string) (delegate *Delegate) {
	var ok bool
	delegate, ok = mock.Delegates[name]
	if !ok {
		mock.Lock()
		defer mock.Unlock()
		delegate, ok = mock.Delegates[name]
		if !ok {
			delegate = new(Delegate)
			mock.Delegates[name] = delegate
		}
	}

	return
}
