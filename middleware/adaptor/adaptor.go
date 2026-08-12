// Package adaptor translates net/http to fiber and vice versa
package adaptor

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"unsafe"

	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

// FiberHandler wraps net/http handler to fiber handler
func FiberHandler(h http.Handler) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var r http.Request
		if err := fasthttpadaptor.ConvertRequest(c.Context(), &r, true); err != nil {
			return err
		}

		ctx, cancel := context.WithCancel(c.UserContext())
		defer cancel()

		if done := c.Context().Done(); done != nil {
			go func() {
				select {
				case <-done:
					cancel()
				case <-ctx.Done():
				}
			}()
		}

		r = *r.WithContext(ctx)

		w := netHTTPResponseWriter{
			statusCode: http.StatusOK,
			h:          make(http.Header),
			w:          c.Context().Response.BodyWriter(),
		}

		h.ServeHTTP(&w, &r)

		c.Context().Response.SetStatusCode(w.statusCode)
		for k, vv := range w.h {
			for _, v := range vv {
				c.Context().Response.Header.Add(k, v)
			}
		}

		return nil
	}
}

// FiberHandlerFunc wraps net/http handler func to fiber handler
func FiberHandlerFunc(h http.HandlerFunc) fiber.Handler {
	return FiberHandler(h)
}

// FiberApp wraps fiber app to net/http handler
func FiberApp(app *fiber.App) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			host string
			port string
			err  error
		)
		if host, port, err = net.SplitHostPort(r.RemoteAddr); err != nil {
			host = r.RemoteAddr
			port = ""
		}
		req := fasthttp.AcquireRequest()
		defer fasthttp.ReleaseRequest(req)

		req.Header.SetMethod(r.Method)
		req.SetRequestURI(r.RequestURI)
		req.Header.SetHost(r.Host)
		for k, v := range r.Header {
			for _, vv := range v {
				req.Header.Add(k, vv)
			}
		}

		if r.Body != nil {
			body, err := io.ReadAll(r.Body)
			if err == nil {
				req.SetBody(body)
			}
		}

		var conn net.Conn
		if r.TLS != nil {
			conn = &tlsConn{conn: r.Body, tls: r.TLS}
		} else {
			conn = &nonTLSConn{conn: r.Body}
		}

		var ctx fasthttp.RequestCtx
		ctx.Init(req, &addr{host: host, port: port}, nil)
		if r.TLS != nil {
			ctx.Response.Addr = &addr{host: host, port: port}
		}

		app.Handler()(&ctx)

		ctx.Response.Header.VisitAll(func(k, v []byte) {
			w.Header().Add(string(k), string(v))
		})

		w.WriteHeader(ctx.Response.StatusCode())

		_, _ = w.Write(ctx.Response.Body())
	})
}

// HTTPHandler wraps fiber handler to net/http handler
func HTTPHandler(h fiber.Handler) http.Handler {
	return HTTPHandlerFunc(h)
}

// HTTPHandlerFunc wraps fiber handler to net/http handler func
func HTTPHandlerFunc(h fiber.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		app := fiber.New()
		app.Get("/*", h)
		FiberApp(app).ServeHTTP(w, r)
	}
}

type netHTTPResponseWriter struct {
	statusCode int
	h          http.Header
	w          io.Writer
}

func (w *netHTTPResponseWriter) Header() http.Header {
	if w.h == nil {
		w.h = make(http.Header)
	}
	return w.h
}

func (w *netHTTPResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

func (w *netHTTPResponseWriter) Write(p []byte) (int, error) {
	return w.w.Write(p)
}

type tlsConn struct {
	net.Conn
	conn io.Reader
	tls  *tls.ConnectionState
}

func (c *tlsConn) Read(b []byte) (int, error) {
	return c.conn.Read(b)
}

func (c *tlsConn) Write(b []byte) (int, error) {
	return 0, nil
}

func (c *tlsConn) Close() error {
	return nil
}

func (c *tlsConn) LocalAddr() net.Addr {
	return nil
}

func (c *tlsConn) RemoteAddr() net.Addr {
	return nil
}

func (c *tlsConn) Handshake() error {
	return nil
}

func (c *tlsConn) ConnectionState() tls.ConnectionState {
	return *c.tls
}

type nonTLSConn struct {
	net.Conn
	conn io.Reader
}

func (c *nonTLSConn) Read(b []byte) (int, error) {
	return c.conn.Read(b)
}

func (c *nonTLSConn) Write(b []byte) (int, error) {
	return 0, nil
}

func (c *nonTLSConn) Close() error {
	return nil
}

func (c *nonTLSConn) LocalAddr() net.Addr {
	return nil
}

func (c *nonTLSConn) RemoteAddr() net.Addr {
	return nil
}

type addr struct {
	net.Addr
	host string
	port string
}

func (a *addr) Network() string {
	return "tcp"
}

func (a *addr) String() string {
	return net.JoinHostPort(a.host, a.port)
}
