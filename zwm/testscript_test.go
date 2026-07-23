package main

import (
	"os"
	"os/exec"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

var testRealGit string

func TestMain(testMain *testing.M) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		os.Exit(1)
	}
	testRealGit = gitPath
	testscript.Main(testMain, map[string]func(){
		"gh":      fakeGitHub,
		"git":     fakeGit,
		"realgit": runRealGit,
		"zellij":  fakeZellij,
	})
}

func TestScripts(t *testing.T) {
	testscript.Run(t, scriptParams())
}

func TestRealGit(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Files:               []string{"testdata/script/real-git.txt"},
		RequireExplicitExec: true,
		Setup:               setupScript,
		Cmds:                scriptCommands(),
	})
}

func scriptParams() testscript.Params {
	return testscript.Params{
		Dir:                 "testdata/script",
		RequireExplicitExec: true,
		Setup:               setupScript,
		Cmds:                scriptCommands(),
	}
}

func scriptCommands() map[string]func(*testscript.TestScript, bool, []string) {
	return map[string]func(*testscript.TestScript, bool, []string){
		"assert-exit": assertExit,
		"assert-log":  assertLog,
	}
}

func mainExit(code int) {
	os.Exit(code)
}
