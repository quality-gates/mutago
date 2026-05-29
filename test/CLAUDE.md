# test

`mutator.go` is the shared test helper for all mutator packages. It uses `go/printer.Fprint` (not `gofmt`) to render mutated ASTs, then compares them to golden files in `testdata/`.

`go/printer` writes `func main()\t{}` with a tab before `{}`. `gofmt` changes that to a space. If you change the rendering here, the golden files in `testdata/` must be regenerated to match — and vice versa, never gofmt the golden files.
