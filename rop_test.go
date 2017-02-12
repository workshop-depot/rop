package rop

import "testing"

func TestSample(t *testing.T) {
	errBoom := Error(`BOOM!`)

	var step1 ProcessorFunc = func(input Payload) Payload {
		return input
	}
	var step2 ProcessorFunc = func(input Payload) Payload {
		if input.Payload != `2nd` {
			return Payload{Err: errBoom}
		}
		return input
	}
	var step3 ProcessorFunc = func(input Payload) Payload {
		if input.Err != nil {
			return Payload{Err: input.Err}
		}
		return input
	}
	c := Chain(step1, step2, step3)

	res := c.Process(Payload{Payload: `1st`})
	if res.Err != errBoom {
		t.Fail()
	}

	res = c.Process(Payload{Payload: `2nd`})
	if res.Payload != `2nd` {
		t.Fail()
	}
}
