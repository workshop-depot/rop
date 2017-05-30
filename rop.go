package rop

import (
	"errors"
	"fmt"
)

//-----------------------------------------------------------------------------

// Domain messages/events/errors
type Domain struct {
	// Msg contains messages/events
	Msg []error `json:"msg"`
	Err []error `json:"err"`
}

//-----------------------------------------------------------------------------
// Result

// Result passes through the railway
type Result struct {
	Res interface{} `json:"res"`
	Domain
}

// NewResult creates new Result
func NewResult() *Result {
	r := new(Result)
	return r
}

func (r *Result) mergePrev(prev *Result) *Result {
	if prev == nil {
		return r
	}
	if len(prev.Msg) > 0 {
		r.Msg = append(prev.Msg, r.Msg...)
	}
	if len(prev.Err) > 0 {
		r.Err = append(prev.Err, r.Err...)
	}
	return r
}

// AddErr adds an error
func (r *Result) AddErr(err error) *Result {
	if err != nil {
		r.Err = append(r.Err, err)
	}
	return r
}

// AddMsg adds a (domain) message/event
func (r *Result) AddMsg(msg error) *Result {
	if msg != nil {
		r.Msg = append(r.Msg, msg)
	}
	return r
}

//-----------------------------------------------------------------------------
// Handling steps

// ResultWriter for writing output Result
type ResultWriter interface {
	Write(Result)
	Prev() Result
}

type resWriter struct {
	prev Result
}

func newResWriter() *resWriter {
	return &resWriter{}
}

func (w *resWriter) Write(in Result) {
	// current := &in
	// current.mergePrev(&w.prev)
	// w.prev = *current
	w.prev = in
}

func (w resWriter) Prev() Result { return w.prev }

// Handler represents each step in the railway
type Handler interface {
	Handle(Result, ResultWriter)
}

// HandlerFunc for using a function as a Handler
type HandlerFunc func(Result, ResultWriter)

// Handle implements Handler
func (hf HandlerFunc) Handle(r Result, w ResultWriter) {
	hf(r, w)
}

//-----------------------------------------------------------------------------

// Errors
var (
	ErrNoProcessor = errors.New(`ErrNoProcessor`)
	ErrInvalidFunc = errors.New(`ErrInvalidFunc`)
)

func adapt(f interface{}) func(Handler) Handler {
	if f == nil {
		return nil
	}
	var middleware func(Handler) Handler

	switch current := f.(type) {
	case func(Result, ResultWriter):
		middleware = func(next Handler) Handler {
			return HandlerFunc(func(r Result, w ResultWriter) {
				if len(w.Prev().Err) == 0 {
					current(r, w)
				}
				if next != nil {
					next.Handle(w.Prev(), w)
				}
			})
		}
	case func(Handler) Handler:
		middleware = current
	case func(Result) Result:
		middleware = func(next Handler) Handler {
			return HandlerFunc(func(r Result, w ResultWriter) {
				// supervisory functions which will always get called
				// if len(w.Prev().Err) == 0 { ...
				res := current(r)
				rbuff := w.Prev()
				res.mergePrev(&rbuff)
				w.Write(res)
				// log.Printf("%+v", w.Prev())
				if next != nil {
					next.Handle(w.Prev(), w)
				}
			})
		}
	case func() Handler:
		middleware = func(next Handler) Handler {
			return HandlerFunc(func(r Result, w ResultWriter) {
				if len(w.Prev().Err) == 0 {
					current().Handle(r, w)
				}
				if next != nil {
					next.Handle(w.Prev(), w)
				}
			})
		}
	case func(Result, ResultWriter, Handler):
		middleware = func(next Handler) Handler {
			return HandlerFunc(func(r Result, w ResultWriter) {
				if len(w.Prev().Err) == 0 {
					current(r, w, next)
				}
			})
		}
	case Handler:
		middleware = func(next Handler) Handler {
			return HandlerFunc(func(r Result, w ResultWriter) {
				if len(w.Prev().Err) == 0 {
					current.Handle(r, w)
				}
				if next != nil {
					next.Handle(w.Prev(), w)
				}
			})
		}
	case func(interface{}) (interface{}, error):
		middleware = func(next Handler) Handler {
			return HandlerFunc(func(r Result, w ResultWriter) {
				if len(w.Prev().Err) == 0 {
					res, err := current(r.Res)
					rs := NewResult()
					rs.Res = res
					rs.AddErr(err)
					rbuff := w.Prev()
					rs.mergePrev(&rbuff)
					w.Write(*rs)
					// log.Printf("%+v", w.Prev())
				}
				if next != nil {
					next.Handle(w.Prev(), w)
				}
			})
		}
	case func(interface{}) error:
		middleware = func(next Handler) Handler {
			return HandlerFunc(func(r Result, w ResultWriter) {
				if len(w.Prev().Err) == 0 {
					err := current(r.Res)
					rs := NewResult()
					rs.Res = r.Res
					rs.AddErr(err)
					rbuff := w.Prev()
					rs.mergePrev(&rbuff)
					w.Write(*rs)
				}
				if next != nil {
					next.Handle(w.Prev(), w)
				}
			})
		}
	case func(interface{}) interface{}:
		middleware = func(next Handler) Handler {
			return HandlerFunc(func(r Result, w ResultWriter) {
				if len(w.Prev().Err) == 0 {
					res := current(r.Res)
					rs := NewResult()
					rs.Res = res
					rbuff := w.Prev()
					rs.mergePrev(&rbuff)
					w.Write(*rs)
				}
				if next != nil {
					next.Handle(w.Prev(), w)
				}
			})
		}
	case func(interface{}):
		middleware = func(next Handler) Handler {
			return HandlerFunc(func(r Result, w ResultWriter) {
				if len(w.Prev().Err) == 0 {
					current(r.Res)
					rs := NewResult()
					rs.Res = r.Res
					rbuff := w.Prev()
					rs.mergePrev(&rbuff)
					w.Write(*rs)
				}
				if next != nil {
					next.Handle(w.Prev(), w)
				}
			})
		}
	default:
		middleware = func(next Handler) Handler {
			return HandlerFunc(func(r Result, w ResultWriter) {
				rs := NewResult()
				rs.Res = r.Res
				rs.AddMsg(fmt.Errorf("error: railway invalid func type %T", current))
				rs.AddErr(ErrInvalidFunc)
				rbuff := w.Prev()
				rs.mergePrev(&rbuff)
				rbuff = *rs
				w.Write(*rs)
				if next != nil {
					next.Handle(w.Prev(), w)
				}
			})
		}
	}

	return middleware
}

//-----------------------------------------------------------------------------

func nop(r Result, w ResultWriter) { w.Write(r) }

// Chain create a chain of processors - our railway segments. Valid signatures
// are supervisory functions which will always get called:
//	func(Result) Result
// and non-supervisory functions which won't get called if there are any errors:
//	func(interface{}) (interface{}, error)
//	func(interface{}) error
//	func(interface{}) interface{}
//	func(interface{})
// otherwise an ErrInvalidFunc error would be added to errors passed in the
// chain. Also the invalid type would be presented inside Msg list.
func Chain(processors ...interface{}) func(Result) Result {
	return func(input Result) Result {
		if len(processors) == 0 {
			r := NewResult()
			r.AddErr(ErrNoProcessor)
			r.mergePrev(&input)
			return *r
		}

		var adapted []func(Handler) Handler
		for _, vf := range processors {
			adapted = append(adapted, adapt(vf))
		}
		nop := HandlerFunc(nop)
		var final Handler = nop

		for i := len(adapted) - 1; i >= 0; i-- {
			mid := adapted[i]

			if mid == nil {
				continue
			}

			if next := mid(final); next != nil {
				final = next
			} else {
				final = nop
			}
		}

		if final == nil {
			final = nop
		}

		w := newResWriter()
		final.Handle(input, w)
		return w.Prev()
	}
}

// // // PipeChain runs a chain concurrently & after the in channel gets closed
// // // and depleted, it closes the out channel.
// // func PipeChain(in <-chan Result, processors ...interface{}) <-chan Result {
// // 	out := make(chan Result)

// // 	p := Chain(processors...)
// // 	go func() {
// // 		defer close(out)
// // 		for v := range in {
// // 			out <- p(v)
// // 		}
// // 	}()
// // 	return out
// // }
