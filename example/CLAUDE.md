# example

`example.go` is the live target for integration tests in `cmd/mutago/main_test.go`. The tests invoke the mutation binary against this file and sometimes leave it with a mutation applied.

After running `go test ./cmd/mutago/`, always run:

```bash
git restore example/example.go
```
