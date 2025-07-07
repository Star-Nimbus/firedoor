package conditions

// String returns the string representation of the condition
func (c Condition) String() string {
	return string(c)
}

// String returns the string representation of the reason
func (r Reason) String() string {
	return string(r)
}

// String returns the string representation of the message
func (m Message) String() string {
	return string(m)
}

// String returns the string representation of the operation
func (o Operation) String() string {
	return string(o)
}
