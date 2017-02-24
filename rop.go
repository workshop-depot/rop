package rop

// PipeChain runs a chain concurrently & after the in channel gets closed
// and depleted, it closes the out channel.
func PipeChain(in <-chan Payload, processors ...Processor) <-chan Payload {
	out := make(chan Payload)
	go func() {
		defer close(out)
		p := Chain(processors...)
		for v := range in {
			out <- p.Process(v)
		}
	}()
	return out
}

// Chain create a chain of processors - our railway segments
func Chain(processors ...Processor) Processor {
	return ProcessorFunc(func(input Payload) Payload {
		if len(processors) == 0 {
			return Payload{Err: ErrNoProcessor}
		}

		unit := ProcessorFunc(func(input1 Payload) Payload { return input1 })
		var final Processor = unit

		for _, prc := range processors {
			if prc == nil {
				continue
			}
			final = reduce(prc, final)
		}

		return final.Process(input)
	})
}

func reduce(p1, p2 Processor) Processor {
	return ProcessorFunc(func(input Payload) Payload {
		return p1.Process(p2.Process(input))
	})
}

// ProcessorFunc is a helper for easy conversion of functions with this
// func(Payload) Payload signature to a Processor interface
type ProcessorFunc func(Payload) Payload

// Process implements Processor
func (x ProcessorFunc) Process(input Payload) Payload {
	return x(input)
}

// Processor is a segment of our railway
type Processor interface {
	Process(Payload) Payload
}

// Payload is what goes around and comes around on which IMO is the same easy
// returning (result, err) in Go; but also we use it for applying the same
// pattern to the input
type Payload struct {
	Payload interface{} `json:"payload"`
	Err     error       `json:"err"`
}

// Constants
const (
	ErrNoProcessor = Error(`ErrNoProcessor`)
)

// Error helps with creating easy, comparable errors
type Error string

func (v Error) Error() string { return string(v) }
