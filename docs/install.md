# Installation

## Requirements

- Go 1.26 or later

## Homebrew (macOS)

```bash
brew install quality-gates/tap/mutago
```

After `brew tap quality-gates/tap`, you can also install with `brew install mutago`.

## Install the binary (Go)

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
