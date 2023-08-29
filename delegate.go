package mock

import "sync"

type Delegate struct {
	sync.Mutex
	Callables
	callCount int
}

func (d *Delegate) Append(callable ...Callable) Callables {
	d.Lock()
	defer d.Unlock()
	d.Callables = d.Callables.Append(callable...)
	return d.Callables
}

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
