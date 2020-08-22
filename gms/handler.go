package gms

import (
	"strings"

	"toast.cafe/x/gemini"
)

// A Handler responds to a gemini request.
//
// ServeGem should write reply headers and data into Ctx.Res and then return.
// Returning signals that the request is finished; it is not valid to modify Ctx after returning.
type Handler interface {
	ServeGem(*gemini.Ctx)
}

// HandlerFunc is an adapater that allows using standalone functions as gemini Handlers
type HandlerFunc func(*gemini.Ctx)

// ServeGem calls f(ctx)
func (f HandlerFunc) ServeGem(ctx *gemini.Ctx) {
	f(ctx)
}

// RedirectHandler generates a Handler that will send a redirect to the given address with the given code
//
// If it returns nil, it means that your code is not valid.
func RedirectHandler(url string, code gemini.Status) HandlerFunc {
	if code < 30 || code >= 40 {
		return nil
	}
	return func(ctx *gemini.Ctx) {
		ctx.Res.Status = code
		ctx.Res.SetMeta(url)
	}
}

// StripPrefix calls the given handler, but with the prefix stripped from the request url
func StripPrefix(prefix string, next Handler) HandlerFunc {
	return func(ctx *gemini.Ctx) {
		ctx.Req.URL.Path = strings.TrimPrefix(ctx.Req.URL.Path, prefix)
		next.ServeGem(ctx)
	}
}
