package main

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestApplicationProcessLifecycle(t *testing.T) {
	if os.Getenv("SOCIETAS_PROCESS_SMOKE") == "1" {
		main()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestApplicationProcessLifecycle$")
	cmd.Env = append(os.Environ(), "SOCIETAS_PROCESS_SMOKE=1", "SOCIETAS_ADDR=127.0.0.1:0")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start application process: %v", err)
	}
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})

	readyLine := make(chan string, 1)
	readErr := make(chan error, 1)
	go func() {
		line, err := bufio.NewReader(stdout).ReadString('\n')
		if err != nil {
			readErr <- err
			return
		}
		readyLine <- strings.TrimSpace(line)
	}()

	var line string
	select {
	case line = <-readyLine:
	case err := <-readErr:
		t.Fatalf("read application readiness: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("application did not report readiness")
	}
	const prefix = "listening on "
	if !strings.HasPrefix(line, prefix) {
		t.Fatalf("readiness message = %q, want prefix %q", line, prefix)
	}
	url := fmt.Sprintf("http://%s/health", strings.TrimPrefix(line, prefix))
	if err := waitForProcessHealth(url); err != nil {
		t.Fatal(err)
	}

	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatalf("signal graceful shutdown: %v", err)
	}
	stopped := make(chan error, 1)
	go func() { stopped <- cmd.Wait() }()
	select {
	case err := <-stopped:
		if err != nil {
			t.Fatalf("application exited with error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("application did not shut down after interrupt")
	}
}

func waitForProcessHealth(url string) error {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		res, err := http.Get(url)
		if err == nil {
			_ = res.Body.Close()
			if res.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("health endpoint did not become ready: %s", url)
}
