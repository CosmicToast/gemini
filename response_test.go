package gemini_test

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"toast.cafe/x/gemini"
)

func randStatus() gemini.Status {
	rand.Seed(time.Now().UTC().UnixNano())
	return gemini.Status(rand.Intn(100-10) + 10)
}

func randMeta(l int) []byte {
	buf := randBody(l) // l can be > 1024, we're testing
	return bytes.ReplaceAll(buf, []byte{'\r'}, []byte{'\n'})
}

func randBody(l int) []byte {
	rand.Seed(time.Now().UTC().UnixNano())
	buf := make([]byte, l)
	rand.Read(buf)
	return buf
}

func combine(status gemini.Status, meta, body []byte) []byte {
	l := 5 + len(meta) + len(body)
	nums := strconv.Itoa(int(status))
	buf := make([]byte, l)
	copy(buf, nums)
	if buf[2] == 0 { // allow up to 3 chars in status, for testing against >=100
		buf[2] = ' '
	}
	copy(buf[3:], meta)
	copy(buf[len(meta)+3:len(meta)+5], []byte("\r\n"))
	copy(buf[len(meta)+5:], body)
	return buf
}

func randReader(ml int, bl int) []byte {
	if ml > gemini.MaxMeta {
		ml = gemini.MaxMeta
	}
	num := randStatus()
	meta := randMeta(ml)
	body := randBody(bl)
	return combine(num, meta, body)
}

type respTest struct {
	status gemini.Status
	meta   []byte
	body   []byte
	err    error
}

func (v respTest) Test(tb testing.TB, r *gemini.Response, err error) {
	if !errors.Is(err, v.err) {
		tb.Errorf("expected error %q, instead found %q", v.err, errors.Unwrap(err))
	}
	if err != nil {
		return // we errored out, so let's not check the rest
	}

	if r.Status != v.status {
		tb.Errorf("expected status %d, instead found %d", v.status, r.Status)
	}
	m := string(v.meta)
	if r.Meta() != m {
		tb.Errorf("expected meta %q, instead found %q", v.meta, r.Meta())
	}
	s := string(v.body)
	body, _ := r.Body()
	if body != s {
		tb.Errorf("expected body %q, instead found %q", s, body)
	}
}

// baseline test set, the same for both reading and writing!
var testresp = []respTest{
	{randStatus(), randMeta(gemini.MaxMeta), randBody(4096), nil},       // normal scenario
	{randStatus(), randMeta(gemini.MaxMeta), nil, nil},                  // empty body
	{randStatus(), nil, randBody(4096), nil},                            // empty meta
	{randStatus(), nil, randBody(0), nil},                               // empty both
	{randStatus(), randMeta(gemini.MaxMeta + 1), nil, gemini.ErrHeader}, // meta too long
	{1, randMeta(gemini.MaxMeta), randBody(4096), gemini.ErrHeader},     // status too short
	{100, randMeta(gemini.MaxMeta), randBody(4096), gemini.ErrHeader},   // status too long
}

func TestReadResponse(t *testing.T) {
	var r gemini.Response
	var b bytes.Reader
	for _, v := range testresp {
		buf := combine(v.status, v.meta, v.body)
		b.Reset(buf)
		r.Reset()

		err := r.FromReader(&b)
		v.Test(t, &r, err)
	}
}

// general write response testing
func TestWriteResponse(t *testing.T) {
	for _, v := range testresp {
		r, err := gemini.NewResponse(v.status, string(v.meta))
		if r != nil { // sometimes we return nil from NewResponse because creating it failed
			r.Write(v.body)
			r.Flush()
		}
		v.Test(t, r, err)
	}
}

// if you are doing proxying, you can just io.Copy the bodies across
func TestCopyResponse(t *testing.T) {
	for _, v := range testresp {
		r1, err1 := gemini.NewResponse(v.status, string(v.meta))
		r2, err2 := gemini.NewResponse(v.status, string(v.meta))
		if r1 != nil {
			r1.Write(v.body)
			r1.Flush()
			io.Copy(r2, r1)
			r2.Flush()
		}

		v.Test(t, r1, err1)
		v.Test(t, r2, err2)
	}
}

// fuzzing

func TestFuzzReadResponse(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	var r gemini.Response
	var b bytes.Reader
	var test respTest

	// round 1: normal
	for i := 0; i < 100000; i++ {
		test.status = randStatus()
		test.meta = randMeta(gemini.MaxMeta)
		test.body = randBody(4096)
		// keep error as nil

		buf := combine(test.status, test.meta, test.body)
		b.Reset(buf)
		r.Reset()

		err := r.FromReader(&b)
		test.Test(t, &r, err)
	}

	// round 2: empty meta
	for i := 0; i < 100000; i++ {
		test.status = randStatus()
		test.meta = nil
		test.body = randBody(4096)
		// keep error as nil

		buf := combine(test.status, test.meta, test.body)
		b.Reset(buf)
		r.Reset()

		err := r.FromReader(&b)
		test.Test(t, &r, err)
	}

	// round 3: empty body
	for i := 0; i < 100000; i++ {
		test.status = randStatus()
		test.meta = randMeta(gemini.MaxMeta)
		test.body = nil
		// keep error as nil

		buf := combine(test.status, test.meta, test.body)
		b.Reset(buf)
		r.Reset()

		err := r.FromReader(&b)
		test.Test(t, &r, err)
	}

	// round 4: empty both
	for i := 0; i < 100000; i++ {
		test.status = randStatus()
		test.meta = nil
		test.body = nil
		// keep error as nil

		buf := combine(test.status, test.meta, test.body)
		b.Reset(buf)
		r.Reset()

		err := r.FromReader(&b)
		test.Test(t, &r, err)
	}
}

// ---- benchmarks

func BenchmarkReadHeader(b *testing.B) {
	buf := randReader(gemini.MaxMeta, 0)
	rdr := bytes.NewReader(buf)
	r := new(gemini.Response)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := r.FromReader(rdr)

		b.StopTimer()
		{ // timer stopped
			if err != nil {
				b.Error(err)
			}
			r.Reset()
			rdr.Reset(buf)
		}
		b.StartTimer()
	}
}

func BenchmarkReadResponse(b *testing.B) {
	buf := randReader(gemini.MaxMeta, 20000) // arbitrary large body
	rdr := bytes.NewReader(buf)
	r := new(gemini.Response)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := r.FromReader(rdr)
		ioutil.ReadAll(r)

		b.StopTimer()
		{ // timer stopped
			if err != nil {
				b.Error(err)
			}
			r.Reset()
			rdr.Reset(buf)
		}
		b.StartTimer()
	}
}
