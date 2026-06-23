package sonic

import (
	"encoding/json"
)

// Node is a minimal shim for the values returned by sonic.Get / sonic.GetFromString.
// Callers in bifrost only use Exists(), Raw() and String().
type Node struct {
	exists bool
	value  interface{}
}

// Exists reports whether the requested path was found.
func (n Node) Exists() bool {
	return n.exists
}

// Raw returns the JSON text of the node.
func (n Node) Raw() (string, error) {
	if !n.exists {
		return "", nil
	}
	b, err := json.Marshal(n.value)
	return string(b), err
}

// String returns the string representation of the node.
// For JSON strings the quotes are stripped; for other values the raw JSON is returned.
func (n Node) String() string {
	if !n.exists {
		return ""
	}
	if s, ok := n.value.(string); ok {
		return s
	}
	b, _ := json.Marshal(n.value)
	return string(b)
}

// Get searches and locates the given path from src json.
// Each path arg must be an integer (array index) or a string (object key).
func Get(src []byte, path ...interface{}) (Node, error) {
	return GetFromString(string(src), path...)
}

// GetCopyFromString is the same as GetFromString (copy is irrelevant for the std-json shim).
func GetCopyFromString(src string, path ...interface{}) (Node, error) {
	return GetFromString(src, path...)
}

// GetWithOptions searches and locates the given path from src json; options are ignored.
func GetWithOptions(src []byte, opts interface{}, path ...interface{}) (Node, error) {
	return GetFromString(string(src), path...)
}

// GetFromString searches and locates the given path from src json.
// Each path arg must be an integer (array index) or a string (object key).
func GetFromString(src string, path ...interface{}) (Node, error) {
	var root interface{}
	if err := json.Unmarshal([]byte(src), &root); err != nil {
		return Node{}, err
	}
	v, ok := walkPath(root, path)
	return Node{exists: ok, value: v}, nil
}

func walkPath(v interface{}, path []interface{}) (interface{}, bool) {
	if len(path) == 0 {
		return v, true
	}
	switch cur := path[0].(type) {
	case string:
		m, ok := v.(map[string]interface{})
		if !ok {
			return nil, false
		}
		child, ok := m[cur]
		if !ok {
			return nil, false
		}
		return walkPath(child, path[1:])
	case int:
		arr, ok := v.([]interface{})
		if !ok || cur < 0 || cur >= len(arr) {
			return nil, false
		}
		return walkPath(arr[cur], path[1:])
	case int64:
		arr, ok := v.([]interface{})
		if !ok || cur < 0 || int(cur) >= len(arr) {
			return nil, false
		}
		return walkPath(arr[cur], path[1:])
	default:
		// Unsupported path type; return not-found.
		return nil, false
	}
}

// Pretouch and PretouchMany are no-ops in the std-json fallback.
// They are not part of the API interface and are not called by bifrost.
func Pretouch(vt interface{}, opts ...interface{}) error { return nil }
func PretouchMany(vts []interface{}, opts ...interface{}) error { return nil }

// NoCopyRawMessage is a placeholder type exported by sonic; not used by bifrost.
type NoCopyRawMessage []byte

