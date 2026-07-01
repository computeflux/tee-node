/*
 * Pure-Go fallback shim for github.com/bytedance/sonic.
 *
 * The real sonic package uses a JIT decoder/encoder and SIMD routines that
 * allocate/execute runtime-generated machine code. This is incompatible with
 * SGX/EGo enclaves, where dynamic executable pages are not available, causing
 * segmentation faults inside enclaves (observed as a crash during Anthropic
 * request parsing).
 *
 * This shim exposes the subset of sonic's public API used by
 * github.com/maximhq/bifrost/core and delegates everything to encoding/json.
 */

package sonic

import (
	"io"
)

const (
	// UseStdJSON indicates the fallback implementation (encoding/json).
	UseStdJSON = iota
	// UseSonicJSON indicates the real sonic implementation.
	UseSonicJSON
)

// APIKind matches sonic's public constant.
const APIKind = UseStdJSON

const apiKind = UseStdJSON

// Config mirrors sonic's public Config struct so that callers can set the
// fields they already use (SortMapKeys, UseInt64, etc.).
type Config struct {
	EscapeHTML              bool
	SortMapKeys             bool
	CompactMarshaler        bool
	NoQuoteTextMarshaler    bool
	NoNullSliceOrMap        bool
	UseInt64                bool
	UseNumber               bool
	UseUnicodeErrors        bool
	DisallowUnknownFields   bool
	CopyString              bool
	ValidateString          bool
	NoValidateJSONMarshaler bool
	NoValidateJSONSkip      bool
	NoEncoderNewline        bool
	EncodeNullForInfOrNan   bool
	CaseSensitive           bool
}

// API mirrors sonic's public API interface.
type API interface {
	MarshalToString(v any) (string, error)
	Marshal(v any) ([]byte, error)
	MarshalIndent(v any, prefix, indent string) ([]byte, error)
	UnmarshalFromString(str string, v any) error
	Unmarshal(data []byte, v any) error
	NewEncoder(writer io.Writer) Encoder
	NewDecoder(reader io.Reader) Decoder
	Valid(data []byte) bool
}

// Encoder mirrors sonic's public Encoder interface.
type Encoder interface {
	Encode(val any) error
	SetEscapeHTML(on bool)
	SetIndent(prefix, indent string)
}

// Decoder mirrors sonic's public Decoder interface.
type Decoder interface {
	Decode(val any) error
	Buffered() io.Reader
	DisallowUnknownFields()
	More() bool
	UseNumber()
}

var (
	// ConfigDefault is the default config (EscapeHTML = false, matching sonic).
	ConfigDefault = Config{}.Froze()

	// ConfigStd is the standard config (EscapeHTML/SortMapKeys, matching sonic).
	ConfigStd = Config{
		EscapeHTML:       true,
		SortMapKeys:      true,
		CompactMarshaler: true,
	}.Froze()
)
