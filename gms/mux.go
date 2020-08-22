package gms

import "toast.cafe/x/gemini"

var (
	_ Mux = (*domainMux)(nil)
	_ Mux = (*pathMux)(nil)
)

// A mux is a gemini multiplexer
//
// The job of a multiplexer is to determine the appropriate function to call given a context.
// A mux does this transparently after setup.
type Mux interface {
	Handler                   // a mux acts as a handler, but actually calls different handlers under the hood
	Register(string, Handler) // Register lets you register various under-the-hood handlers
}

// common mux memory structure
type commonMux struct {
	kv map[string]Handler
}

// this is a common initialization pattern
func newCommonMux(v Handler) *commonMux {
	var mux commonMux
	mux.kv = make(map[string]Handler)
	mux.kv[""] = v
	return &mux
}

// exact match with fallback
func commonExact(m map[string]Handler, k string) Handler {
	if val, ok := m[k]; ok {
		return val
	}
	return m[""] // if you erase it by hand, shame on you
}

// ---- by domain

type domainMux commonMux

// DomainMux initializes a mux that performs muxing based on the requested domain.
//
// The passed handler will be used as the "fallback" handler, in case no precise key is found.
func DomainMux(v Handler) *domainMux {
	return (*domainMux)(newCommonMux(v))
}

// Register registers a given exact domain to call the specific handler.
//
// Note that domain matching is exact, and that the empty string will overwrite the "fallback" handler.
func (mux *domainMux) Register(k string, v Handler) {
	mux.kv[k] = v
}

// ServeGem passes on to the handler registered for a given domain name, else the fallback handler.
func (mux *domainMux) ServeGem(ctx *gemini.Ctx) {
	commonExact(mux.kv, ctx.Req.String()).ServeGem(ctx)
}

// ---- by exact path

type pathMux commonMux

// PathMux initializes a mux that performs muxing based on the exact requested path.
//
// The passed handler will be used as the "fallback" handler, in case there are no exact matches.
func PathMux(v Handler) *pathMux {
	return (*pathMux)(newCommonMux(v))
}

// Register registers a given exact path to call the specific handler.
//
// Note that domain matching is exact, and that the empty string will overwrite the "fallback" handler.
func (mux *pathMux) Register(k string, v Handler) {
	mux.kv[k] = v
}

// ServeGem passes on the handler registered for a given path, else the fallback handler.
func (mux *pathMux) ServeGem(ctx *gemini.Ctx) {
	commonExact(mux.kv, ctx.Req.Path()).ServeGem(ctx)
}
