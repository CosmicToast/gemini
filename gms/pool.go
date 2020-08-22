package gms

import (
	"sync"

	"toast.cafe/x/gemini"
)

var ctxPool = sync.Pool{
	New: func() interface{} {
		return new(gemini.Ctx)
	},
}

var resPool = sync.Pool{
	New: func() interface{} {
		return new(gemini.Response)
	},
}
