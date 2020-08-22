package gemini

import (
	"crypto/x509"
	"fmt"
	"io"
)

// Ctx is a gemini context, for both clients and servers
type Ctx struct {
	Req *Request
	Res *Response

	ClientCerts []*x509.Certificate // server only
	ServerCerts []*x509.Certificate // client only
}

// NewRequestCtx constructs a request context from a string
func NewRequestCtx(s string) (c *Ctx, e error) {
	c = new(Ctx)
	r, e := ParseRequest(s)
	c.Req = r
	return
}

// Status returns the status of the response
func (ctx *Ctx) Status() Status {
	return ctx.Res.Status
}

// Meta returns the meta field of the response header
func (ctx *Ctx) Meta() string {
	return ctx.Res.Meta()
}

// WriteHeader efficiently writes the response header to the writer
func (ctx *Ctx) WriteHeader(w io.Writer) (e error) {
	_, e = fmt.Fprintf(w, "%d %s\r\n", ctx.Status(), ctx.Meta())
	return
}

// Header returns the response header as a string, without the \r\n
func (ctx *Ctx) Header() string {
	return fmt.Sprintf("%d %s", ctx.Status(), ctx.Meta())
}
