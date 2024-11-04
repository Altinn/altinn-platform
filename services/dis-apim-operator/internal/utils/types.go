package utils

// ToPointer gets the pointer of a value.
func ToPointer[T any](t T) *T {
	return &t
}
