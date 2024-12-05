package utils

func Bytes(s string) []byte {
	return []byte(s)
}

func Must[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}

	return value
}

func Slice[T any](args ...T) []T {
	return args
}
