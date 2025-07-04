package utils

// ToPointer gets the pointer of a value.
func ToPointer[T any](t T) *T {
	return &t
}

func PointerValueEqual[T comparable](a *T, b *T) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}
