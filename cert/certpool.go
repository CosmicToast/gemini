package cert

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"
)

// CertPool implements a managed certificate pool.
//
// Specifically, it implements a string-keyed repository of certificates that one can "Get" from.
// If a given certificate is missing or expired from the stateful directory, it will be automatically generated in the background.
// The pool also comes with an optional routine that will automatically renew certificates when they expire without requiring an explicit "Get".
//
// The pool restricts you to a single certificate per host/domain/key.
// This is to simplify the managed nature thereof, as well as increase privacy for the self-signed-only use-case.
//
// Note that this pool is biased towards usage for gemini servers.
// Feel free to adapt it to your own needs, though, as it is licensed under the unlicense!
type CertPool struct {
	store string
	certs map[string]*tls.Certificate
	files []os.FileInfo
}

// NewStore will open the given directory, creating it if needed.
func NewStore(directory string) (*CertPool, error) {
	_, err := os.Open(directory) // err is *os.PathError
	if err != nil {
		err = os.MkdirAll(directory, 0700)
		if err != nil {
			return nil, err
		}
	}
	var pool CertPool
	pool.store = directory
	err = pool.reparseDir()
	return &pool, err
}

// OpenStore will open the given directory, creating it if needed, and preload all of the certificate pairs it can find from it.
func OpenStore(directory string) (c *CertPool, err error) {
	c, err = NewStore(directory)
	if err != nil {
		return
	}

	for _, v := range c.files {
		if v.IsDir() { // idk why there'd be a directory there but you never know
			continue
		}
		name := v.Name()
		suff := path.Ext(name)
		if suff == ".key" { // only look at keys, directory might also store known hosts
			c.load(strings.TrimSuffix(name, suff)) // ignore err, we just continue
		}
	}

	return
}

// Get will get you a certificate for the given name.
//
// Name must be a valid filename component.
// The order of operations is:
// 1. if there is a cached cert, check for expiry (go to 4).
// 2. if there is no cached cert, try to load one from the store, and check for expiry on success (go to 4).
// 3. if there is no cert in the store, generate one and save it in the store. return it.
// 4. if the cert is expired, goto 3, else return it
func (c *CertPool) Get(name string) (*tls.Certificate, error) {
	if cert, ok := c.certs[name]; ok {
		if !expired(cert) {
			return cert, nil
		}
		// it's expired
		if err := c.generate(name); err != nil {
			return nil, err // we have failed
		}
		return c.Get(name)
	}

	err := c.load(name)
	if err == nil {
		return c.Get(name)
	}

	err = c.generate(name)
	if err == nil {
		return c.Get(name)
	}

	return nil, err
}

func leaf(cert *tls.Certificate) (*x509.Certificate, error) {
	if cert.Leaf != nil {
		return cert.Leaf, nil
	}
	return x509.ParseCertificate(cert.Certificate[0])
}

// returns true on error
// check c.Leaf in those cases
// golang team expose (*tls.Certificate).leaf() challenge 2020
func expired(cert *tls.Certificate) bool {
	xcert, err := leaf(cert)
	if err != nil {
		return true
	}
	exp := xcert.NotAfter
	now := time.Now()
	return now.After(exp)
}

func (c *CertPool) generate(name string) error {
	return nil // TODO: implement me pls
}

func (c *CertPool) load(name string) error {
	keypath := path.Join(c.store, name+".key")
	certpath := path.Join(c.store, name+".pem")

	cert, err := tls.LoadX509KeyPair(certpath, keypath)
	if err != nil {
		return err
	}

	c.certs[name] = &cert
	return nil
}

func (c *CertPool) reparseDir() (err error) {
	c.files, err = ioutil.ReadDir(c.store)
	return
}
