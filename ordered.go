package vermock

type ordered struct {
	inOrder bool
	ordinal uint
}

func orderedOption[T any](inOrder bool, options []Option[T]) Option[T] {
	return func(key *T) {
		mock := registry[key]
		defer func(restore bool) {
			mock.inOrder = restore
		}(mock.inOrder)
		mock.inOrder = inOrder
		for _, option := range options {
			option(key)
		}
	}
}

func ExpectInOrder[T any](options ...Option[T]) Option[T] {
	return orderedOption(true, options)
}

func ExpectAnyOrder[T any](options ...Option[T]) Option[T] {
	return orderedOption(false, options)
}
