package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestInterruptLifecycleHelper(t *testing.T) {
	if os.Getenv("MUTAGO_INTERRUPT_HELPER") != "1" {
		return
	}
	interrupts := newRunInterrupts()
	defer interrupts.close()
	fmt.Println("ready")
	<-interrupts.ctx.Done()
	fmt.Println("cancelled")
	select {}
}

func TestInterruptLifecycle(t *testing.T) {
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		t.Run(sig.String(), func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestInterruptLifecycleHelper$")
			cmd.Env = append(os.Environ(), "MUTAGO_INTERRUPT_HELPER=1")
			out, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
			lines := make(chan string, 2)
			go func() {
				s := bufio.NewScanner(out)
				for s.Scan() {
					lines <- s.Text()
				}
			}()
			waitLine := func(want string) {
				t.Helper()
				select {
				case got := <-lines:
					if got != want {
						t.Fatalf("got %q, want %q", got, want)
					}
				case <-time.After(5 * time.Second):
					t.Fatalf("timed out waiting for %q", want)
				}
			}
			waitLine("ready")
			if err := cmd.Process.Signal(sig); err != nil {
				t.Fatal(err)
			}
			waitLine("cancelled")
			if err := cmd.Process.Signal(sig); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			select {
			case err := <-done:
				require.Error(t, err, "second %v did not terminate by signal", sig)
				status, ok := cmd.ProcessState.Sys().(syscall.WaitStatus)
				require.True(t, ok, "process did not return a wait status")
				require.True(t, status.Signaled(), "process did not terminate by signal")
				require.Equal(t, sig, status.Signal(), "second signal did not terminate process")
			case <-time.After(5 * time.Second):
				t.Fatal("second signal did not terminate process")
			}
		})
	}
}
