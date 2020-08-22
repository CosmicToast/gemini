package gemini

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"path"
	"runtime"
	"unsafe"
)

// Request represents a gemini request
type Request struct {
	*url.URL
}

// Parse parses the request in the context of the current one
//
// This is particularly convenient for relative links in clients.
func (r *Request) Parse(ref string) (*Request, error) {
	var e error
	var out Request
	out.URL, e = r.URL.Parse(ref)
	return &out, e
}

// Canonicalize tries to prepare a request for sending
//
// It will insert defaults as per the spec.
// It returns false if the request is irrecoverable.
func (r *Request) Canonicalize() bool {
	u := r.URL
	// disallowed components
	u.Opaque = ""
	u.User = nil

	// irrecoverable
	if u.Hostname() == "" || // authority section is required
		!path.IsAbs(u.Path) { // sent paths must be absolute
		return false
	}

	// apply defaults
	if u.Scheme == "" {
		u.Scheme = "gemini"
	}
	if u.Port() == "" {
		u.Host += ":1965"
	}

	return true
}

// ReadRequest constructs a request from a reader, and expects a \r\n
func ReadRequest(r io.Reader) (*Request, error) {
	// we can over-read because there is no request body in gemini
	buf := make([]byte, MaxURL+2) // \r\n
	_, e1 := r.Read(buf)          // io.Reader says we should process n before looking at errors

	l := bytes.Index(buf, []byte("\r\n"))
	if l < 0 {
		return nil, fmt.Errorf("%w: no \\r\\n in %d bytes", ErrRequest, MaxURL+2)
	}

	u := buf[:l] // the url without the \r\n
	runtime.KeepAlive(u)
	rr, e2 := ParseRequest(*(*string)(unsafe.Pointer(&u)))

	if e1 != nil {
		return rr, e1
	}
	return rr, e2
}

// ParseRequest constructs a request from a string without an \r\n
func ParseRequest(s string) (*Request, error) {
	var e error
	var out Request
	out.URL, e = url.Parse(s)
	return &out, e
}

func (r *Request) Host() string {
	return r.URL.Hostname()
}

func (r *Request) Path() string {
	return r.URL.EscapedPath()
}

func (r *Request) Query() string {
	return r.URL.RawQuery
}

func (r *Request) Fragment() string {
	return r.URL.Fragment
}

func (r *Request) String() string {
	return r.URL.String()
}
