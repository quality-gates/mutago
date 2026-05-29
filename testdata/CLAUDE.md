# testdata

These are golden files for mutator tests. **Do not run `gofmt` or `gofmt -s` on them.**

The test helper (`test/mutator.go`) checks mutator output using `go/printer.Fprint`, which writes `func main()\t{}` with a tab before `{}`. Running `gofmt` changes that tab to a space, which breaks every mutator test that uses these files.

These files are excluded from the goreportcard quality gate, so leaving them unformatted is fine.
