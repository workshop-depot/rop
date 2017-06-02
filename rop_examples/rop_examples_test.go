package rop_examples

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/dc0d/rop"
	"github.com/stretchr/testify/assert"
)

func recoveryMiddleware(next rop.Handler) rop.Handler {
	return rop.HandlerFunc(func(r *rop.Result, w rop.ResultWriter) {
		defer func() {
			if e := recover(); e != nil {
				r.AddErr(fmt.Errorf("RECOVERED"))
			}
		}()
		if next != nil {
			next.Handle(w.Last(), w)
		}
	})
}

var currentYear = time.Now().Year()

func divideByZero(v interface{}) interface{} {
	zero := time.Now().Year() - currentYear
	if n, ok := v.(int); ok {
		return n / zero
	}
	return 0
}

func dumpErrorsMiddleware(next rop.Handler) rop.Handler {
	return rop.HandlerFunc(func(r *rop.Result, w rop.ResultWriter) {
		if next != nil {
			next.Handle(w.Last(), w)
		}
		// if debug
		// dump w.Last().Failure ...
	})
}

func loggerMiddleware(next rop.Handler) rop.Handler {
	return rop.HandlerFunc(func(r *rop.Result, w rop.ResultWriter) {
		if next != nil {
			next.Handle(w.Last(), w)
		}
		// ... logging
		w.Write(w.Last().AddMsg(errors.New("logged")))
	})
}

func TestExampleRecovery(t *testing.T) {
	c := rop.Chain(nil, dumpErrorsMiddleware, loggerMiddleware, recoveryMiddleware, divideByZero)
	r := rop.NewResult(nil)
	r.SetValue(1)
	res := c(r)
	assert.Len(t, res.Failure, 1)
	assert.Equal(t, "RECOVERED", res.Failure[0].Error())
}
