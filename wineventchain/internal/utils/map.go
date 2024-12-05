package utils

func Keys[T comparable, U any](m map[T]U) []T {
	keys := make([]T, len(m))

	i := 0
	for key := range m {
		keys[i] = key
		i++
	}

	return keys
}
