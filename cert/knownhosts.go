package cert

import (
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"toast.cafe/x/gemini"
)

// Host represents a known host.
type Host struct {
	Expiry      time.Time `json:",omitempty"` // missing expiry = no expiry
	Fingerprint string
}

// KnownHosts implements a certificate verifier backed by a "known hosts" file.
//
// We don't import gmc to avoid a circular dependency cycle in case gmc wants to import cert.
type KnownHosts struct {
	path  string          // the path to the file
	hosts map[string]Host // cache
}

// NewKnownHosts creates a KnownHost certificate verifier backed by the file at path.
func NewKnownHosts(path string) (*KnownHosts, error) {
	dir := filepath.Dir(path)
	if fi, _ := os.Stat(dir); fi == nil || !fi.IsDir() {
		if fi != nil { // exists, is not dir
			return nil, nil // TODO: this is an error
		}
		// does not exist, create
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return nil, err
		}
	}

	res := KnownHosts{path, nil}
	err := res.Load()
	if err != nil { // file didn't exist, just initialize the map
		res.hosts = make(map[string]Host)
	}

	return &res, nil
}

// VerifyCert verifies a host's certificate against a list of known certificates.
//
// This implementation returns nil if the certificate in the known hosts list is expired, replacing the version in the file.
// Users should check for the Error: if it isn't gemini.ErrCert then it was an issue in saving the file, and will likely happen again on repeat attempts.
// Note that a zero value for expiry means no expiry. This lets you permanently trust certificates by manipulating the known hosts file by hand.
func (r *KnownHosts) VerifyCert(host string, certs []*x509.Certificate) error {
	if val, ok := r.hosts[host]; ok {
		if !(val.Expiry.IsZero() || val.Expiry.After(time.Now())) { // not expired
			//if tn := time.Now(); !tn.After(val.Expiry) { // it's not expired, check fingerprint
			fc := Fingerprint(certs[0]) // only consider the leaf certificate
			if fc != val.Fingerprint {
				return fmt.Errorf("%w: non-expired known fingerprint (%s) does not match the one found (%s)", gemini.ErrCert, val.Fingerprint, fc)
			}
		}
	} // it was expired, update - same action as when we don't have it
	r.hosts[host] = Host{certs[0].NotAfter, Fingerprint(certs[0])}
	return r.Save()
}

// Load will forcibly drop the cache and load it from the file at the path.
func (r *KnownHosts) Load() error {
	b, err := ioutil.ReadFile(r.path)
	if err != nil {
		return err
	}

	r.hosts = make(map[string]Host) // drop old cache
	err = json.Unmarshal(b, r.hosts)
	return err
}

// Save will forcibly save the known hosts file.
func (r *KnownHosts) Save() error {
	b, err := json.Marshal(r.hosts)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(r.path, b, 0600)
	return err
}

// Fingerprint is a convenience function for fingerprinting a certificate.
//
// Implementation details may change on MAJOR release version updates.
// The current implementation returns a base64-encoded padless sha512 sum of the raw certificate.
func Fingerprint(cert *x509.Certificate) string {
	buf := sha512.Sum512(cert.Raw)
	return base64.RawStdEncoding.EncodeToString(buf[:])
}
