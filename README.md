# rop
### Minimal Railway Oriented Programming for Go

For reading more about Railway Oriented Programming, look  [here](http://fsharpforfunandprofit.com/rop/).

I've been tempted to create some async interfaces and the like, but what was doing was actually re-abstracting Go's syntax, which provides all sort of tools to make things concurrent.

It's easy to construct reusable chains, with stateless `Processor`s and even (to some extent) graphs of functionalities.

Returning `(result, err)` is actually a Go idiom. The reason for defining a `Payload` struct which does the same, is that it makes it possible to send the result over a channel; it's pretty much just a tuple.

Status: I've used it in bunch of in-house code bases & 'am happy with it. Of-course one coucld with full blown enterprisy frameworks! But I like how Go simplifies things to the bone! For a sample see the test file.
