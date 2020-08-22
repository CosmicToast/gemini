package gemini

import (
	//"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"

	//"reflect"
	"strconv"
	"unsafe"
)

type bytesWriter interface {
	io.Writer
	io.StringWriter
	Bytes() []byte
}

type responseHeader struct {
	Status Status
	meta   []byte
}

// Meta returns the meta field
func (r *responseHeader) Meta() string {
	return *(*string)(unsafe.Pointer(&r.meta))
}

// MetaBytes returns the underlying byte slice
//
// Note that this uses copy, so you can't break the ResponseHeader.
// It is recommended to use Meta() instead.
func (r *responseHeader) MetaBytes() []byte {
	out := make([]byte, len(r.meta))
	copy(out, r.meta)
	return out
}

// SetMeta sets the meta of the response header to the contents of the string
func (r *responseHeader) SetMeta(s string) { // TODO: make sure this is safe
	r.meta = *(*[]byte)(unsafe.Pointer(&s)) // we promise we won't mutate the string
}

// SetMetaBytes sets the meta of the response header to the contents of the byte slice
//
// Note that this uses copy, so that you can keep using the byte slice afterwards.
// It is recommended to use SetMeta() instead.
func (r *responseHeader) SetMetaBytes(b []byte) {
	r.meta = make([]byte, len(b))
	copy(r.meta, b)
}

// Header returns the raw header (for sending) as a byte slice
func (r *responseHeader) Header() []byte {
	out := make([]byte, len(r.meta)+5)
	copy(out, strconv.Itoa(int(r.Status)))
	out[2] = ' '
	copy(out[3:], r.meta)
	copy(out[len(r.meta)+3:], []byte("\r\n"))
	return out
}

// Response represents a gemini response
type Response struct {
	responseHeader
	body []byte

	read    bool // called Read()?
	flushed bool // called Flush()?
	reader  io.Reader
	writer  bytesWriter
}

// NewResponse is a response initializer for use by servers
//
// Note: your meta should not include the terminating \r\n.
func NewResponse(status Status, meta string) (*Response, error) {
	if len(meta) > 1024 {
		return nil, fmt.Errorf("%w: meta length > 1024 bytes", ErrHeader)
	}
	if status < 10 || status >= 100 {
		return nil, fmt.Errorf("%w: invalid response (%d)", ErrHeader, status)
	}
	var r Response

	b := make([]byte, len(meta))
	copy(b, meta)
	r.meta = b
	r.Status = status

	r.ServerPrepare()
	return &r, nil
}

// ServerPrepare prepares a response for usage by a server
//
// This involves primarily populating the writer.
// Note that this pre-assumes an empty response.
func (r *Response) ServerPrepare() {
	var builder bytes.Buffer
	r.writer = &builder
}

// Reset resets the response to be reused
func (r *Response) Reset() {
Reset: // do we need to reset anything?
	switch {
	case r.Status != 0:
		r.Status = 0
		goto Reset
	case r.meta != nil:
		r.meta = nil
		goto Reset
	case r.body != nil:
		r.body = nil
		goto Reset
	case r.read:
		r.read = false
		goto Reset
	case r.reader != nil:
		r.reader = nil
		goto Reset
	case r.writer != nil:
		r.writer = nil
	}
}

// FromReader populates a response from a reader
//
// The reader should include the header line - this is meant to be used by clients.
func (r *Response) FromReader(reader io.Reader) error {
	buf := make([]byte, MaxMeta+5) // exact size of the largest valid header

	n, err := reader.Read(buf) // io.Reader says we should process n before looking at errors
	if n < 4 {                 // for type safety below, cases where crlf is too far we catch elsewhere
		return fmt.Errorf("%w: not enough data in the reader for a valid header", ErrHeader)
	}

	// read status
	r.Status = fastAtoi(buf[:2])
	if r.Status > 99 || r.Status < 10 {
		return fmt.Errorf("%w: status corrupted: %d", ErrHeader, r.Status)
	}
	if buf[2] != ' ' {
		return fmt.Errorf("%w: %c is not a space", ErrHeader, buf[2])
	}

	// read meta
	delim := bytes.Index(buf[3:n], []byte("\r\n")) + 3
	if delim == 2 { // -1 + 3 = 2
		return fmt.Errorf("%w: no \\r\\n in %d bytes", ErrHeader, MaxMeta+5)
	}

	r.meta = buf[3:delim]
	delim += 2 // skip \r\n

	// everything else is the body
	// technically I could do my own buffering and avoid a copy later, but I'm lazy
	r.reader = io.MultiReader(bytes.NewReader(buf[delim:n]), reader) // io.MultiReader does a copy

	if err != io.EOF {
		return err
	}
	return nil
}

// Read allows you to stream the response (body) data
//
// Note that Read() is only valid for responses created from a reader.
// Note that calling Read() invalidates your ability to call Body() later.
func (r *Response) Read(b []byte) (int, error) {
	r.read = true
	return r.reader.Read(b)
}

// Write allows you to stream the body into the response
func (r *Response) Write(b []byte) (int, error) {
	if r.flushed {
		return 0, ErrFlush
	}
	return r.writer.Write(b)
}

// WriteString allows you to stream the body into the response
func (r *Response) WriteString(s string) (int, error) {
	if r.flushed {
		return 0, ErrFlush
	}
	return r.writer.WriteString(s)
}

// Flush flushes the writer from Write() into the body, making Body() callable
//
// Note that once Flushed, you can no longer call Write(), but gain the ability to call Read()
func (r *Response) Flush() {
	r.flushed = true
	r.body = r.writer.Bytes()
	r.reader = bytes.NewReader(r.body)
}

// Body returns the body of the response as a string
//
// Clients: note that you cannot call this after calling Read().
// Servers: note that this will not be valid until you call Write+Flush or SetBody*.
func (r *Response) Body() (out string, err error) {
BodyRet:
	if r.body != nil {
		return *(*string)(unsafe.Pointer(&r.body)), err
	}

	// client
	if r.reader != nil {
		if r.read { // we already ran a Read()
			return "", ErrRead
		}
		r.body, err = ioutil.ReadAll(r.reader)
		goto BodyRet
	}
	return "", nil
}

// ---- util

// read-only atoi version that runs against [2]byte and doesn't allocate
// meant to be inlined, no warnings on failure so make sure output is sane
func fastAtoi(b []byte) Status {
	return Status((10 * (b[0] - '0')) + (b[1] - '0'))
}
