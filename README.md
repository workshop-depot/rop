# rop
### Minimal Railway Oriented Programming for Go

For reading more about Railway Oriented Programming, look  [here](http://fsharpforfunandprofit.com/rop/).

It's easy to construct reusable chains, with stateless `Processor`s and even (to some extent) graphs of functionalities.

A sequential chain of `Processor`s can be made like:

```go
c := Chain(step1, step2, step3)

res := c.Process(Payload{...})
if res.Err != nil {
    // attend to error
}
```

Returning `(result, err)` is actually a Go idiom. The reason for defining a `Payload` struct which does the same, is that it makes it possible to send the result over a channel; it's pretty much just a tuple.

Also a chain of processors can run cuncurrently, employing `PipeChain` functions and channels:

```go
in := make(chan Payload)

go func() {
    defer close(in) // we close it when we are done

    // send to in channel
}()

out := PipeChain(in, step1, step2, step3)
for res := range out {
    // consuming out channel
}
// out channel will get closed when in channel 
// gets depleted & closed, and ranging over out will stop.
```

Status: I've used it in bunch of in-house code bases & 'am happy with it. Of-course one coucld with full blown enterprisy frameworks! But I like how Go simplifies things to the bone! For a sample see the test file.
