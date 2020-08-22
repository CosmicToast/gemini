package gms

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"

	"toast.cafe/x/gemini"
)

type Logger interface {
	// Printf must have the same semantics as log.Printf, including the sync
	Printf(string, ...interface{})
}

type Server struct {
	// TCP address to listen on, defaults to :1965
	Addr      string
	logger    Logger
	TLSConfig *tls.Config
	handler   Handler // TODO: use a default handler?
}

var DefaultServer = &Server{
	Addr: ":1965",
	TLSConfig: &tls.Config{
		MinVersion: tls.VersionTLS12,
	},
}

func (s *Server) log(fmt string, args ...interface{}) {
	if s.logger != nil {
		s.logger.Printf(fmt, args)
	}
}

func (s *Server) Serve() error {
	l, err := tls.Listen("tcp", s.Addr, s.TLSConfig)
	if err != nil {
		return err
	}
	defer l.Close()

	for { // listening loop
		conn, err := l.Accept()
		if err != nil {
			s.log("error while accepting connection: %s", err)
		}

		// handle the connection concurrently
		go func(c net.Conn) {
			defer c.Close()

			ctx := &gemini.Ctx{}
			ctx.Req, err = gemini.ReadRequest(c)
			if err != nil {
				fmt.Fprintf(c, "%d\r\n", gemini.StatusBadRequest)
			}

			// prepare response
			ctx.Res = resPool.Get().(*gemini.Response)
			defer resPool.Put(ctx.Res)
			defer ctx.Res.Reset()
			ctx.Res.ServerPrepare()

			// mux it
			defer func() {
				if r := recover(); r != nil {
					s.log("panic while handling connection: %s", r)
				}
			}()
			s.handler.ServeGem(ctx)
			ctx.Res.Flush()

			// write it
			fmt.Fprintf(c, "%d %s\r\n", ctx.Status, ctx.Meta())
			io.Copy(c, ctx.Res)
		}(conn)
	}
}
