package utils

func ToPointer[T any](t T) *T {
	return &t
}
