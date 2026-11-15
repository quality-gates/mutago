package main

import (
	"os"
	"strings"
	"testing"

	"github.com/jessevdk/go-flags"
	"github.com/quality-gates/mutago/v2/internal/models"
)

func TestVerifyCLIEarlyExitPrecedence(t *testing.T) {
	old := os.Stdout
	defer func() { os.Stdout = old }()
	for bits := 0; bits < 16; bits++ {
		opts := models.NewOptions()
		opts.General.Version = bits&1 != 0
		opts.General.Help = bits&2 != 0
		opts.Mutator.ListMutators = bits&4 != 0
		completion := bits&8 != 0
		if completion {
			t.Setenv("GO_FLAGS_COMPLETION", "1")
		} else {
			t.Setenv("GO_FLAGS_COMPLETION", "")
		}
		p := flags.NewNamedParser("mutago", flags.None)
		if _, err := p.AddGroup("mutago", "mutago arguments", opts); err != nil {
			t.Fatal(err)
		}
		output, err := os.CreateTemp(t.TempDir(), "output")
		if err != nil {
			t.Fatal(err)
		}
		os.Stdout = output
		handled, code := handleEarlyExitFlags(opts, p, []string{"target"})
		if err := output.Close(); err != nil {
			t.Fatal(err)
		}
		os.Stdout = old
		data, err := os.ReadFile(output.Name())
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		expected := "none"
		switch {
		case opts.General.Version:
			expected = "version"
		case opts.General.Help && !completion:
			expected = "help"
		case opts.Mutator.ListMutators:
			expected = "list"
		}
		actual := "none"
		if handled {
			if code != returnOk {
				t.Fatalf("precedence bits=%d: exit %d", bits, code)
			}
			switch {
			case strings.HasPrefix(text, "mutago "):
				actual = "version"
			case strings.HasPrefix(text, "Usage:"):
				actual = "help"
			case strings.Contains(text, "arithmetic"):
				actual = "list"
			default:
				t.Fatalf("precedence bits=%d: unexpected output %q", bits, text)
			}
		}
		if actual != expected {
			t.Fatalf("precedence bits=%d: got %s want %s", bits, actual, expected)
		}
	}
}

func TestVerifyCLIHandledBeforeErrors(t *testing.T) {
	oldOut, oldErr := os.Stdout, os.Stderr
	defer func() { os.Stdout, os.Stderr = oldOut, oldErr }()
	for _, args := range [][]string{
		{"--version", "--not-a-real-flag"},
		{"--help", "--not-a-real-flag"},
		{"--help", "--config", "/path/that/does/not/exist"},
	} {
		file, err := os.CreateTemp(t.TempDir(), "output")
		if err != nil {
			t.Fatal(err)
		}
		os.Stdout, os.Stderr = file, file
		handled, code := checkArguments(args, models.NewOptions())
		os.Stdout, os.Stderr = oldOut, oldErr
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		if !handled || code != returnOk {
			t.Fatalf("early exit was not handled before errors for %v: %v %d", args, handled, code)
		}
	}
}
