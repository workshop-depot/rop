package rop

import (
	"fmt"
)

// PipeChain runs a chain concurrently & after the in channel gets closed
// and depleted, it closes the out channel.
func PipeChain(in <-chan Result, processors ...interface{}) <-chan Result {
	out := make(chan Result)

	p := Chain(processors...)
	go func() {
		defer close(out)
		for v := range in {
			out <- p(v)
		}
	}()
	return out
}

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

		unit := func(in Result) Result { return in }
		var final = unit

		for _, prc := range processors {
			if prc == nil {
				continue
			}
			var next = adapt(prc)
			final = reduce(next, final)
		}

		return final(input)
	}
}

func reduce(p1, p2 func(Result) Result) func(Result) Result {
	return func(in Result) Result {
		return p1(p2(in))
	}
}

func adapt(f interface{}) func(Result) Result {
	var rf func(Result) Result
	switch xf := f.(type) {
	// supervisory function; will always get called
	case func(Result) Result:
		rf = xf

	// non-supervisory functions will not get called if there sre any errors
	case func(interface{}) (interface{}, error):
		rf = func(in Result) Result {
			if len(in.Err) > 0 {
				return in
			}
			res, err := xf(in.Res)
			r := NewResult()
			r.Res = res
			r.AddErr(err)
			r.mergePrev(&in)
			return *r
		}
	case func(interface{}) error:
		rf = func(in Result) Result {
			if len(in.Err) > 0 {
				return in
			}
			err := xf(in.Res)
			r := NewResult()
			r.Res = in.Res
			r.AddErr(err)
			r.mergePrev(&in)
			return *r
		}
	case func(interface{}) interface{}:
		rf = func(in Result) Result {
			if len(in.Err) > 0 {
				return in
			}
			res := xf(in.Res)
			r := NewResult()
			r.Res = res
			r.mergePrev(&in)
			return *r
		}
	case func(interface{}):
		rf = func(in Result) Result {
			if len(in.Err) > 0 {
				return in
			}
			xf(in.Res)
			r := NewResult()
			r.Res = in.Res
			r.mergePrev(&in)
			return *r
		}
	default:
		rf = func(in Result) Result {
			r := NewResult()
			r.Res = in.Res
			r.AddMsg(Error(fmt.Sprintf("error: railway invalid func type %T", xf)))
			r.AddErr(ErrInvalidFunc)
			r.mergePrev(&in)
			return in
		}
	}
	return rf
}

// NewResult creates new Result
func NewResult() *Result {
	r := new(Result)
	return r
}

// Result passes through the railway
type Result struct {
	Res interface{} `json:"res"`
	// Msg contains messages/events
	Msg []error `json:"msg"`
	Err []error `json:"err"`
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

// Error helps with creating easy, comparable errors/messages
type Error string

func (e Error) Error() string { return string(e) }

// Constants
const (
	ErrNoProcessor = Error(`ErrNoProcessor`)
	ErrInvalidFunc = Error(`ErrInvalidFunc`)
)
