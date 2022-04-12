package internal

// Error represents an internal sentinal error which can be defined as a
// constant.
type Error string

func (e Error) Error() string {
	return string(e)
}
