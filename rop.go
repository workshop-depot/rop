package rop

import (
	"errors"
	"fmt"
)

//-----------------------------------------------------------------------------
// Result

// Success .
type Success struct {
	Value    interface{} `json:"res"`
	Messages []error     `json:"msg"`
}

// Result one single Result, passes through the railway
type Result struct {
	*Success `json:"res"`
	Failure  []error `json:"err"`
}

// NewResult creates new Result
func NewResult(value interface{}) *Result {
	r := &Result{}
	if value != nil {
		r.Success = &Success{
			Value: value,
		}
	}
	return r
}

// GetValue .
func (r *Result) GetValue() interface{} {
	if r.Success == nil {
		return nil
	}
	return r.Success.Value
}

// SetValue .
func (r *Result) SetValue(val interface{}) *Result {
	if r.Success == nil {
		r.Success = &Success{}
	}
	r.Success.Value = val
	return r
}

// AddErr adds an error
func (r *Result) AddErr(err ...error) *Result {
	if err == nil {
		return r
	}
	r.Success = nil
	r.Failure = append(r.Failure, err...)
	return r
}

// AddMsg adds a (domain) message/event
func (r *Result) AddMsg(msg error) *Result {
	if msg == nil {
		return r
	}
	if r.Success == nil {
		r.Success = &Success{}
	}
	r.Success.Messages = append(r.Success.Messages, msg)
	return r
}

//-----------------------------------------------------------------------------
// Handling steps

// ResultWriter for writing output Result
type ResultWriter interface {
	Write(*Result)
	Last() *Result
}

// DefaultResultWriter default implementation of ResultWriter
type DefaultResultWriter struct {
	last *Result
}

// NewDefaultResultWriter .
func NewDefaultResultWriter() *DefaultResultWriter {
	return &DefaultResultWriter{}
}

func (w *DefaultResultWriter) Write(in *Result) {
	w.last = in
}

// Last give the previous Result
func (w DefaultResultWriter) Last() *Result { return w.last }

// Handler represents each step in the railway
type Handler interface {
	Handle(*Result, ResultWriter)
}

// HandlerFunc for using a function as a Handler
type HandlerFunc func(*Result, ResultWriter)

// Handle implements Handler
func (hf HandlerFunc) Handle(r *Result, w ResultWriter) {
	hf(r, w)
}

//-----------------------------------------------------------------------------

// Errors
var (
	ErrNoProcessor = errors.New(`ErrNoProcessor`)
)

func adapt(f interface{}) func(Handler) Handler {
	if f == nil {
		return nil
	}
	var middleware func(Handler) Handler

	switch current := f.(type) {
	case func(*Result, ResultWriter):
		middleware = func(next Handler) Handler {
			return HandlerFunc(func(r *Result, w ResultWriter) {
				if w.Last() != nil && len(w.Last().Failure) == 0 {
					current(r, w)
				}
				if next != nil {
					next.Handle(w.Last(), w)
				}
			})
		}
	case func(Handler) Handler:
		middleware = current
	case func(*Result) *Result:
		middleware = func(next Handler) Handler {
			return HandlerFunc(func(r *Result, w ResultWriter) {
				// supervisory functions which will always get called
				// if len(w.Prev().Err) == 0 { ...
				res := current(r)
				w.Write(res)
				if next != nil {
					next.Handle(w.Last(), w)
				}
			})
		}
	case func() Handler:
		middleware = func(next Handler) Handler {
			return HandlerFunc(func(r *Result, w ResultWriter) {
				if w.Last() != nil && len(w.Last().Failure) == 0 {
					current().Handle(r, w)
				}
				if next != nil {
					next.Handle(w.Last(), w)
				}
			})
		}
	case func(*Result, ResultWriter, Handler):
		middleware = func(next Handler) Handler {
			return HandlerFunc(func(r *Result, w ResultWriter) {
				if w.Last() != nil && len(w.Last().Failure) == 0 {
					current(r, w, next)
				}
			})
		}
	case Handler:
		middleware = func(next Handler) Handler {
			return HandlerFunc(func(r *Result, w ResultWriter) {
				if w.Last() != nil && len(w.Last().Failure) == 0 {
					current.Handle(r, w)
				}
				if next != nil {
					next.Handle(w.Last(), w)
				}
			})
		}
	case func(interface{}) (interface{}, error):
		middleware = func(next Handler) Handler {
			return HandlerFunc(func(r *Result, w ResultWriter) {
				if w.Last() != nil && len(w.Last().Failure) == 0 {
					res, err := current(r.GetValue())
					if err != nil {
						r.AddErr(err)
					} else {
						r.SetValue(res)
					}
					w.Write(r)
				}
				if next != nil {
					next.Handle(w.Last(), w)
				}
			})
		}
	case func(interface{}) error:
		middleware = func(next Handler) Handler {
			return HandlerFunc(func(r *Result, w ResultWriter) {
				if w.Last() != nil && len(w.Last().Failure) == 0 {
					err := current(r.GetValue())
					if err != nil {
						r.AddErr(err)
					}
					w.Write(r)
				}
				if next != nil {
					next.Handle(w.Last(), w)
				}
			})
		}
	case func(interface{}) interface{}:
		middleware = func(next Handler) Handler {
			return HandlerFunc(func(r *Result, w ResultWriter) {
				if w.Last() != nil && len(w.Last().Failure) == 0 {
					res := current(r.GetValue())
					r.SetValue(res)
					w.Write(r)
				}
				if next != nil {
					next.Handle(w.Last(), w)
				}
			})
		}
	case func(interface{}):
		middleware = func(next Handler) Handler {
			return HandlerFunc(func(r *Result, w ResultWriter) {
				if w.Last() != nil && len(w.Last().Failure) == 0 {
					current(r.GetValue())
					w.Write(r)
				}
				if next != nil {
					next.Handle(w.Last(), w)
				}
			})
		}
	default:
		middleware = func(next Handler) Handler {
			return HandlerFunc(func(r *Result, w ResultWriter) {
				r.AddErr(fmt.Errorf("error: railway invalid func type %T", current))
				w.Write(r)
				if next != nil {
					next.Handle(w.Last(), w)
				}
			})
		}
	}

	return middleware
}

//-----------------------------------------------------------------------------

func nop(r *Result, w ResultWriter) { w.Write(r) }

// Chain create a chain of processors - our railway segments. Valid signatures
// are supervisory functions which will always get called:
//	func(*Result) *Result // if there are no panics
//	func(Handler) Handler // always
// Second signature is a middleware with same assumed semantics as in Go's middleware pattern in web apps.
// and non-supervisory functions which won't get called if there are any errors:
//	func(*Result, ResultWriter)
//	func() Handler
//	func(*Result, ResultWriter, Handler)
//	a Handler
//	func(interface{}) (interface{}, error)
//	func(interface{}) error
//	func(interface{}) interface{}
//	func(interface{})
// otherwise an error would be added to errors passed in the
// chain. Also the invalid type would be presented inside the error.
func Chain(rw ResultWriter, processors ...interface{}) func(*Result) *Result {
	return func(input *Result) *Result {
		if len(processors) == 0 {
			r := NewResult(nil)
			r.AddErr(ErrNoProcessor)
			return r
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

		if rw == nil {
			rw = NewDefaultResultWriter()
		}
		rw.Write(input)
		final.Handle(input, rw)
		return rw.Last()
	}
}

// PipeChain runs a chain concurrently & after the in channel gets closed
// and depleted, it closes the out channel.
func PipeChain(rw ResultWriter, in <-chan *Result, processors ...interface{}) <-chan *Result {
	out := make(chan *Result)

	p := Chain(rw, processors...)
	go func() {
		defer close(out)
		for v := range in {
			out <- p(v)
		}
	}()
	return out
}
