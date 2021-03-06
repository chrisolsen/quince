package quincy

import (
	"net/http"

	"golang.org/x/net/context"
	"google.golang.org/appengine"
)

// Middleware is a http.HandlerFunc that also includes a context and url params variables
type Middleware func(context.Context, http.ResponseWriter, *http.Request) context.Context

// HandlerFunc much like the standard http.HandlerFunc, but includes the request context
type HandlerFunc func(context.Context, http.ResponseWriter, *http.Request)

// Handler much like the standard http.Handler, but includes the request context
// in the ServeHTTP method
type Handler interface {
	ServeHTTP(context.Context, http.ResponseWriter, *http.Request)
}

// handler allows the middleware calls to be wrapped up into a Handler interface
type handler struct {
	mw      Middleware
	handler Handler
}

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	c = h.mw(c, w, r)
	if c.Err() == nil {
		h.handler.ServeHTTP(c, w, r)
	}
}

// Q allows a list middleware functions to be created and run
type Q struct {
	fns []Middleware
}

// New initializes the middleware chain with one or more handler functions.
// The returned pointer allows for additional middleware methods to be added or
// for the chain to be run.
//	q := que.New(foo, bar)
func New(fns ...Middleware) *Q {
	q := Q{}
	q.fns = fns
	return &q
}

// Add allows for one or more middleware handler functions to be added to the
// existing chain
//	q := que.New(cors, format)
//	q.Add(auth)
func (q *Q) Add(fns ...Middleware) {
	q.fns = append(q.fns, fns...)
}

// Run executes the handler chain, which is most useful in tests
//	q := que.New(foo, bar)
// 	q.Add(func(c context.Context, w http.ResponseWriter, r *http.Request) {
// 		// perform tests here
// 	})
//  inst := aetest.NewInstance(nil)
// 	r := inst.NewRequest("GET", "/", nil)
// 	w := httpTest.NewRecorder()
// 	c := appengine.NewContext(r)
// 	q.Run(c, w, r)
func (q *Q) Run(c context.Context, w http.ResponseWriter, r *http.Request) {
	chain(q.fns)(c, w, r)
}

// Then returns the chain of existing middleware that includes the final HandlerFunc argument.
//	q := que.New(foo, bar)
//  router.Get("/", q.Then(handleRoot))
func (q *Q) Then(fn HandlerFunc) func(http.ResponseWriter, *http.Request) {
	chn := chain(q.fns)

	return func(w http.ResponseWriter, r *http.Request) {
		c := appengine.NewContext(r)
		c = chn(c, w, r)

		if c.Err() == nil {
			fn(c, w, r)
		}
	}
}

// Handle accepts a Handler interface and returns the chain of existing middleware
// that includes the final Handler argument.
//	q := que.New(foo, bar)
//  router.Get("/", q.Then(handleRoot))
func (q *Q) Handle(h Handler) http.Handler {
	mw := chain(q.fns)
	return handler{mw: mw, handler: h}
}

// converts the middleware slice into a series of middleware functions and returns
// a reference to the first middleware item in the chain
func chain(fns []Middleware) Middleware {
	var next Middleware
	var count = len(fns)
	for i := count - 1; i >= 0; i-- {
		next = link(fns[i], next)
	}

	// if there is no middleware a non-nil function is required to allow the final
	// handler function to be called
	if count == 0 {
		next = func(c context.Context, w http.ResponseWriter, r *http.Request) context.Context {
			return c
		}
	}

	return next
}

// links the two middleware functions to allow the first to call the next on completion
func link(current, next Middleware) Middleware {
	return func(c context.Context, w http.ResponseWriter, r *http.Request) context.Context {
		c = current(c, w, r)
		if c.Err() != nil {
			return c
		}
		if next != nil {
			c = next(c, w, r)
		}
		return c
	}
}
