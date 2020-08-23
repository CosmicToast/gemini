package gmc

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"toast.cafe/x/gemini"
)

// A CertChecker verifies the validity of a certificate chain relative to a hostname.
//
// VerifyCert takes the hostname and the known certificate chain.
// It should return nil if the certificate is valid for that hostname.
type CertChecker interface {
	VerifyCert(string, []*x509.Certificate) error
}

// CertCheckerFunc is an adapter that allows using standalone functions as a CertChecker
type CertCheckerFunc func(string, []*x509.Certificate) error

func (c CertCheckerFunc) VerifyCert(s string, cs []*x509.Certificate) error {
	return c(s, cs)
}

// Client is a gemini client
type Client struct {
	TLSConfig *tls.Config
	Proxy     string // "" means direct, should include port

	// Checker will be used by this client to verify hostnames.
	//
	// If nil, all certs are considered valid for all hosts.
	Checker CertChecker
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
		host = ctx.Req.Host()
	}

	// get connection
	con, err := tls.Dial("tcp", host, c.TLSConfig)
	if err != nil {
		return err
	}
	ctx.ServerCerts = con.ConnectionState().PeerCertificates

	if c.Checker != nil {
		if err := c.Checker.VerifyCert(host, ctx.ServerCerts); err != nil {
			return fmt.Errorf("VerifyCert returned an error: %w", err)
		}
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

	if canon := ctx.Req.Canonicalize(); !canon {
		return nil, fmt.Errorf("%w: canonicalization failed", gemini.ErrRequest)
	}

	return ctx, c.Do(ctx)
}
