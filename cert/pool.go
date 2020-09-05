package cert

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
	"math/big"
	"os"
	"path"
	"strings"
	"time"
)

// certTemplate is a baseline template used to generate self-signed certs
var certTemplate = x509.Certificate{
	KeyUsage:              x509.KeyUsageDigitalSignature,
	ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	BasicConstraintsValid: true, // enforce maxpathlen 0
}

// how long managed certs will be valid for
const validLength = time.Hour * 24 * 60

// Pool implements a managed certificate pool for servers.
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
type Pool struct {
	store string
	certs map[string]*tls.Certificate
	files []os.FileInfo
}

// NewStore will open the given directory, creating it if needed.
func NewStore(directory string) (*Pool, error) {
	_, err := os.Open(directory) // err is *os.PathError
	if err != nil {
		err = os.MkdirAll(directory, 0700)
		if err != nil {
			return nil, err
		}
	}
	var pool Pool
	pool.store = directory
	err = pool.reparseDir()
	return &pool, err
}

// OpenStore will open the given directory, creating it if needed, and preload all of the certificate pairs it can find from it.
func OpenStore(directory string) (c *Pool, err error) {
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
func (c *Pool) Get(name string) (*tls.Certificate, error) {
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

func (c *Pool) generate(name string) error {
	// serial number
	serialMax := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialMax)
	if err != nil {
		return err // TODO: could not generate serial number
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err // TODO: could not generate ed25519 key
	}

	tmpl := certTemplate // copy
	tmpl.SerialNumber = serial
	tmpl.NotBefore = time.Now()
	tmpl.NotAfter = tmpl.NotBefore.Add(validLength)
	tmpl.DNSNames = []string{name}

	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, pub, priv)
	if err != nil {
		return err // TODO: could not generate certificate
	}

	// marshal + write
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return err // TODO: could not marshal private key
	}

	keypath := path.Join(c.store, name+".key")
	certpath := path.Join(c.store, name+".pem")

	kf, err := os.OpenFile(keypath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err // TODO: could not open {keypath} for writing
	}
	defer kf.Close()
	cf, err := os.OpenFile(certpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err // TODO: could not open {certpath} for writing
	}
	defer cf.Close()

	if err := pem.Encode(kf, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		return err // TODO: could not write/encode private key
	}
	if err := pem.Encode(cf, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
		return err // TODO: could not write/encode certificate
	}

	return nil
}

func (c *Pool) load(name string) error {
	keypath := path.Join(c.store, name+".key")
	certpath := path.Join(c.store, name+".pem")

	cert, err := tls.LoadX509KeyPair(certpath, keypath)
	if err != nil {
		return err
	}

	c.certs[name] = &cert
	return nil
}

func (c *Pool) reparseDir() (err error) {
	c.files, err = ioutil.ReadDir(c.store)
	return
}
