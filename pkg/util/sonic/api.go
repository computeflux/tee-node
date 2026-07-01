package sonic

// Package-level helpers delegate to ConfigDefault.

// Marshal returns the JSON encoding bytes of v.
func Marshal(val any) ([]byte, error) {
	return ConfigDefault.Marshal(val)
}

// MarshalIndent returns the JSON encoding bytes with indentation.
func MarshalIndent(v any, prefix, indent string) ([]byte, error) {
	return ConfigDefault.MarshalIndent(v, prefix, indent)
}

// MarshalString returns the JSON encoding string of v.
func MarshalString(val any) (string, error) {
	return ConfigDefault.MarshalToString(val)
}

// Unmarshal parses JSON-encoded data into v.
func Unmarshal(buf []byte, v any) error {
	return ConfigDefault.Unmarshal(buf, v)
}

// UnmarshalString parses JSON-encoded string into v.
func UnmarshalString(buf string, v any) error {
	return ConfigDefault.UnmarshalFromString(buf, v)
}

// Valid reports whether data is a valid JSON encoding.
func Valid(data []byte) bool {
	return ConfigDefault.Valid(data)
}

// ValidString reports whether s is a valid JSON encoding.
func ValidString(data string) bool {
	return ConfigDefault.Valid([]byte(data))
}
