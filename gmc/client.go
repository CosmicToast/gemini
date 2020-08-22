package gmc

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"toast.cafe/x/gemini"
)

// Client is a gemini client
type Client struct {
	TLSConfig *tls.Config
	Proxy     string // "" means direct, should include port

	// VerifyCert should return true or false on whether or not any cert in the set is valid for a given host
	//
	// If nil, all certs are considered valid for all hosts.
	VerifyCert func(string, []*x509.Certificate) bool
}

// DefaultClient is the default
var DefaultClient = &Client{
	TLSConfig: &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true,
	},
}

func (c *Client) SetCertificates(certs ...tls.Certificate) {
	c.TLSConfig.Certificates = certs
}

// Do performs the request in the context, populating it
func (c *Client) Do(ctx *gemini.Ctx) error {
	host := c.Proxy
	if host == "" {
		host = ctx.Req.Host
	}

	// get connection
	con, err := tls.Dial("tcp", host, c.TLSConfig)
	if err != nil {
		return err
	}
	ctx.ServerCerts = con.ConnectionState().PeerCertificates

	if c.VerifyCert != nil && !c.VerifyCert(host, ctx.ServerCerts) {
		return fmt.Errorf("%w: VerifyCert returned false", gemini.ErrCert)
	}

	// send request
	// TODO: normalize request
	fmt.Fprintf(con, "%s\r\n", ctx.Req)

	// receive response
	ctx.Res = new(gemini.Response)
	err = ctx.Res.FromReader(con)
	return err
}

// Fetch parses your request and returns a populated context
func (c *Client) Fetch(req string) (*gemini.Ctx, error) {
	ctx, err := gemini.NewRequestCtx(req)
	if err != nil {
		return nil, err
	}

	if canon := ctx.Req.Canonicalize(nil); !canon {
		return nil, fmt.Errorf("%w: canonicalization failed", gemini.ErrRequest)
	}

	return ctx, c.Do(ctx)
}
