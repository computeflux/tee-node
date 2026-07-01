package sonic

import (
	"bytes"
	"encoding/json"
	"io"
)

// frozenConfig implements the API interface by delegating to encoding/json.
type frozenConfig struct {
	Config
}

// Froze converts the Config to an API binding.
func (cfg Config) Froze() API {
	return &frozenConfig{Config: cfg}
}

func (cfg *frozenConfig) marshalOptions(val any, prefix, indent string) ([]byte, error) {
	w := bytes.NewBuffer(nil)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(cfg.EscapeHTML)
	enc.SetIndent(prefix, indent)
	if err := enc.Encode(val); err != nil {
		return nil, err
	}
	out := w.Bytes()
	// json.Encoder always appends a trailing newline; trim it to match sonic/json.Marshal.
	if len(out) > 0 && out[len(out)-1] == '\n' {
		out = out[:len(out)-1]
	}
	return out, nil
}

// Marshal implements API.Marshal.
func (cfg *frozenConfig) Marshal(val any) ([]byte, error) {
	if !cfg.EscapeHTML {
		return cfg.marshalOptions(val, "", "")
	}
	return json.Marshal(val)
}

// MarshalToString implements API.MarshalToString.
func (cfg *frozenConfig) MarshalToString(val any) (string, error) {
	out, err := cfg.Marshal(val)
	return string(out), err
}

// MarshalIndent implements API.MarshalIndent.
func (cfg *frozenConfig) MarshalIndent(val any, prefix, indent string) ([]byte, error) {
	if !cfg.EscapeHTML {
		return cfg.marshalOptions(val, prefix, indent)
	}
	return json.MarshalIndent(val, prefix, indent)
}

// UnmarshalFromString implements API.UnmarshalFromString.
func (cfg *frozenConfig) UnmarshalFromString(str string, v any) error {
	r := bytes.NewBufferString(str)
	dec := json.NewDecoder(r)
	if cfg.UseNumber {
		dec.UseNumber()
	}
	if cfg.DisallowUnknownFields {
		dec.DisallowUnknownFields()
	}
	if err := dec.Decode(v); err != nil {
		return err
	}
	// Reject trailing data, matching sonic's strict behaviour.
	offset := dec.InputOffset()
	if t, err := dec.Token(); !(t == nil && err == io.EOF) {
		return &json.SyntaxError{Offset: offset}
	}
	return nil
}

// Unmarshal implements API.Unmarshal.
func (cfg *frozenConfig) Unmarshal(data []byte, v any) error {
	return cfg.UnmarshalFromString(string(data), v)
}

// NewEncoder implements API.NewEncoder.
func (cfg *frozenConfig) NewEncoder(writer io.Writer) Encoder {
	enc := json.NewEncoder(writer)
	if !cfg.EscapeHTML {
		enc.SetEscapeHTML(cfg.EscapeHTML)
	}
	return enc
}

// NewDecoder implements API.NewDecoder.
func (cfg *frozenConfig) NewDecoder(reader io.Reader) Decoder {
	dec := json.NewDecoder(reader)
	if cfg.UseNumber {
		dec.UseNumber()
	}
	if cfg.DisallowUnknownFields {
		dec.DisallowUnknownFields()
	}
	return dec
}

// Valid implements API.Valid.
func (cfg *frozenConfig) Valid(data []byte) bool {
	return json.Valid(data)
}
