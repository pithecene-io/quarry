//nolint:revive // types is a common Go package naming convention
package types

// FileWriteFrame represents a file_write IPC frame.
// This is a single-frame transport for sidecar file uploads via Lode Store.
// It bypasses the Dataset segment/manifest machinery entirely.
//
// Discriminated from event envelopes by Type == "file_write".
// Does not participate in seq numbering or the policy pipeline.
type FileWriteFrame struct {
	// Type is always "file_write" for file write frames.
	Type string `msgpack:"type"`
	// Filename is the target filename (no path separators, no "..").
	Filename string `msgpack:"filename"`
	// ContentType is the MIME content type.
	ContentType string `msgpack:"content_type"`
	// Data is the raw binary data (max 8 MiB).
	Data []byte `msgpack:"data"`
}
