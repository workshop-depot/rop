# rop
### Railway Oriented Programming for Go

For reading more about Railway Oriented Programming, look  [here](http://fsharpforfunandprofit.com/rop/).

It's easy to construct reusable chains, with stateless functions and even (to some extent) graphs of functionalities. 

It's nontheorical essence is:

```
                ________    ________    ________
happy path  --->|      |--->|      |--->|      |---> result
                | func |    | func |    | func |
failure     --->|      |--->|      |--->|      |---> error
                --------    --------    --------
```

As you see it looks like the idiomatic Go pattern of writing functions as `func(TIn) (TOut, error)`, except for the input error from previous step.

A sequential chain of functions can be made like:

```go
c := Chain(nil, step1, step2, step3)

res := c(Result{...})
```

Status: WIP; there are `v1` and `v2` branches, but it's being redesigned to more closely resembles Go's middleware pattern. For example now we can handle panics as we do in web apps.
