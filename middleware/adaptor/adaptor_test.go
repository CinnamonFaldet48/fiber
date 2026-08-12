package adaptor

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
)

func Test_FiberHandler(t *testing.T) {
	t.Parallel()

	expected := "next"
	app := fiber.New()

	app.Get("/test", FiberHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(expected))
	})))

	resp, err := app.Test(httptest.NewRequest("GET", "/test", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, expected, string(body))
}

func Test_FiberHandlerFunc(t *testing.T) {
	t.Parallel()

	expected := "next"
	app := fiber.New()

	app.Get("/test", FiberHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(expected))
	}))

	resp, err := app.Test(httptest.NewRequest("GET", "/test", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, expected, string(body))
}

func Test_FiberApp(t *testing.T) {
	t.Parallel()

	expected := "next"
	app := fiber.New()

	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString(expected)
	})

	handler := FiberApp(app)

	req := httptest.NewRequest("GET", "/test", nil)
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	utils.AssertEqual(t, fiber.StatusOK, resp.Code)
	utils.AssertEqual(t, expected, resp.Body.String())
}

func Test_HTTPHandler(t *testing.T) {
	t.Parallel()

	expected := "next"
	handler := HTTPHandler(func(c *fiber.Ctx) error {
		return c.SendString(expected)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	utils.AssertEqual(t, fiber.StatusOK, resp.Code)
	utils.AssertEqual(t, expected, resp.Body.String())
}

func Test_HTTPHandlerFunc(t *testing.T) {
	t.Parallel()

	expected := "next"
	handler := HTTPHandlerFunc(func(c *fiber.Ctx) error {
		return c.SendString(expected)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	utils.AssertEqual(t, fiber.StatusOK, resp.Code)
	utils.AssertEqual(t, expected, resp.Body.String())
}

func Test_FiberHandler_Context_Cancellation(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	ctxDone := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			close(ctxDone)
		case <-time.After(2 * time.Second):
		}
	})

	app.Get("/test", func(c *fiber.Ctx) error {
		ctx, cancel := context.WithCancel(context.Background())
		c.SetUserContext(ctx)
		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()
		return FiberHandler(handler)(c)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req, 3000)
	utils.AssertEqual(t, nil, err)
	if resp != nil {
		resp.Body.Close()
	}

	select {
	case <-ctxDone:
		// Success
	case <-time.After(3 * time.Second):
		t.Fatal("context was not cancelled in the adapted handler")
	}
}

func Test_FiberHandler_Fasthttp_Context_Cancellation(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	ctxDone := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			close(ctxDone)
		case <-time.After(2 * time.Second):
		}
	})

	app.Get("/test", func(c *fiber.Ctx) error {
		go func() {
			time.Sleep(100 * time.Millisecond)
			c.Context().Cancel()
		}()
		return FiberHandler(handler)(c)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req, 3000)
	utils.AssertEqual(t, nil, err)
	if resp != nil {
		resp.Body.Close()
	}

	select {
	case <-ctxDone:
		// Success
	case <-time.After(3 * time.Second):
		t.Fatal("fasthttp context cancellation was not propagated to the adapted handler")
	}
}
