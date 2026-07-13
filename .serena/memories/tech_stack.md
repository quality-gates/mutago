# Tech Stack

- Go module `github.com/quality-gates/mutago/v2`; Go `1.26.3` from `go.mod`.
- CLI parsing: `github.com/jessevdk/go-flags`; terminal colors: `github.com/fatih/color`; YAML: `gopkg.in/yaml.v3`.
- Tests use standard `testing` plus `github.com/stretchr/testify`.
- CI/local quality tools: `gofmt`, `go vet`, `gocyclo`, `ineffassign`, `messgo`.
- GitHub Actions workflows cover docs, Go Report Card checks, messgo, mutation gates, and security.