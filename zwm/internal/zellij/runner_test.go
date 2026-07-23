package zellij

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"
	"time"
)

func TestSystemRunner_captures_output_when_child_process_writes_streams(t *testing.T) {
	if os.Getenv("ZELLIJ_TEST_RUNNER_MODE") == "output" {
		fmt.Fprint(os.Stdout, "child stdout")
		fmt.Fprint(os.Stderr, "child stderr")
		os.Exit(0)
	}

	// Given
	t.Setenv("ZELLIJ_TEST_RUNNER_MODE", "output")
	runner := SystemRunner{}
	command := Command{
		Name: CommandName(os.Args[0]),
		Args: []string{"-test.run=^TestSystemRunner_captures_output_when_child_process_writes_streams$"},
	}

	// When
	output, err := runner.Run(context.Background(), command)

	// Then
	if err != nil {
		t.Fatalf("run child: %v", err)
	}
	assertEqual(t, Output{Stdout: "child stdout", Stderr: "child stderr"}, output)
}

func TestSystemRunner_terminates_hung_child_when_context_is_cancelled(t *testing.T) {
	const watchdog = 30 * time.Second

	if os.Getenv("ZELLIJ_TEST_RUNNER_MODE") == "hang" {
		connection, err := net.Dial("tcp", os.Getenv("ZELLIJ_TEST_RUNNER_READY_ADDRESS"))
		if err != nil {
			t.Fatalf("notify parent: %v", err)
		}
		defer connection.Close()
		select {}
	}

	// Given
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()
	if err := listener.SetDeadline(time.Now().Add(watchdog)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	t.Setenv("ZELLIJ_TEST_RUNNER_MODE", "hang")
	t.Setenv("ZELLIJ_TEST_RUNNER_READY_ADDRESS", listener.Addr().String())
	timeoutContext, cancelTimeout := context.WithTimeout(context.Background(), watchdog)
	defer cancelTimeout()
	runContext, cancelRun := context.WithCancel(timeoutContext)
	defer cancelRun()
	result := make(chan error, 1)

	// When
	go func() {
		_, runErr := (SystemRunner{}).Run(runContext, Command{
			Name: CommandName(os.Args[0]),
			Args: []string{"-test.run=^TestSystemRunner_terminates_hung_child_when_context_is_cancelled$"},
		})
		result <- runErr
	}()
	connection, err := listener.Accept()
	if err != nil {
		t.Fatalf("wait for child: %v", err)
	}
	connection.Close()
	cancelRun()

	// Then
	select {
	case runErr := <-result:
		if runErr == nil {
			t.Fatal("expected cancelled child to return an error")
		}
	case <-timeoutContext.Done():
		t.Fatal("cancelled child did not terminate")
	}
}
