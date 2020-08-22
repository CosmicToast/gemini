package gemini_test

import (
	"net/url"
	"strings"
	"testing"

	"toast.cafe/x/gemini"
)

// this test is kind of pants, but I just needed to make sure the reader is ok
func TestReadRequest(t *testing.T) {
	url, _ := url.Parse("gemini://some.host:1965/some/path")
	urls := url.String()
	urlsrn := urls + "\r\n"
	rdr := strings.NewReader(urlsrn)

	r1, e1 := gemini.ReadRequest(rdr)
	r2, e2 := gemini.ParseRequest(urls)

	if r1.String() != r2.String() {
		t.Errorf("%q != %q", r1.String(), r2.String())
	}
	if e1 != e2 {
		t.Errorf("%q != %q", e1, e2)
	}

}
