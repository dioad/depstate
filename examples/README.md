# Examples

This directory contains standalone examples of how to use the `depstate` package.

## Examples

- [basic](basic/main.go): Basic usage of the `depstate` package.
- [advanced](advanced/main.go): Advanced usage of the `depstate` package.
- [wait-for](wait-for/main.go): Using `WaitForDependencies` and `GetDependencyStates`.
- [any-met](any-met/main.go): Using `IsDependencyMet` and `WaitForAny`.

## Running Examples

You can run each example using the `go run` command:

```bash
go run examples/basic/main.go
```

Or by navigating to the example directory and building/running it:

```bash
cd examples/basic
go build
./basic
```
