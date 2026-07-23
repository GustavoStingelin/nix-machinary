package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"syscall"
)

type invocation struct {
	Arguments []string
	Directory string
}

func main() {
	directory, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if recordPath := os.Getenv("GIT_HELPER_RECORD"); recordPath != "" {
		record, err := json.Marshal(invocation{Arguments: os.Args[1:], Directory: directory})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if err := os.WriteFile(recordPath, record, 0o600); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
	if os.Getenv("GIT_HELPER_HANG") == "1" {
		parentID := os.Getppid()
		if err := syscall.Kill(parentID, syscall.SIGUSR1); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		select {}
	}
	if exitStatus := os.Getenv("GIT_HELPER_EXIT"); exitStatus != "" {
		fmt.Fprint(os.Stdout, "stdout before failure")
		fmt.Fprint(os.Stderr, "stderr before failure")
		status, err := strconv.Atoi(exitStatus)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		os.Exit(status)
	}
	if encodedOutput := os.Getenv("GIT_HELPER_OUTPUT_BASE64"); encodedOutput != "" {
		output, err := base64.StdEncoding.DecodeString(encodedOutput)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if _, err := os.Stdout.Write(output); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "rev-parse" {
		fmt.Fprintln(os.Stdout, "0123456789abcdef0123456789abcdef01234567")
	}
}
