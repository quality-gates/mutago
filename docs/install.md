# Installation

## Requirements

- Go 1.26 or later

## Install the binary

```bash
go install github.com/quality-gates/mutago/v2/cmd/mutago@latest
```

The binary is placed in `$(go env GOPATH)/bin`. Make sure that directory is on your `PATH`.

## Build from source

```bash
git clone https://github.com/quality-gates/mutago.git
cd mutago
go build -o mutago ./cmd/mutago
```

## Verify

```bash
mutago --help
```
