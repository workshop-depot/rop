# rop
### Railway Oriented Programming for Go

For reading more about Railway Oriented Programming, look  [here](http://fsharpforfunandprofit.com/rop/).

It's easy to construct reusable chains, with stateless functions and even (to some extent) graphs of functionalities.

A sequential chain of functions can be made like:

```go
c := Chain(step1, step2, step3)

res := c(Result{...})
// res.Err contains errors
// res.Msg conains (domain) messages/events
// res.Res contains the computation result
```

Returning `(result, err)` is actually a Go idiom. The reason for defining a `Result` struct which does the same, is that it makes it possible to send the result over a channel; it's pretty much just a tuple.

Also a chain of processors can run cuncurrently, employing `PipeChain` functions and channels:

```go
in := make(chan Result)

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

Status: After V1, saw that embedding domain messages/events inside the `Result` itself makes things much simpler. But this is not a finalized V2 - yet - and might change.