// Copyright (c) 2024 Marcel Heistermann

package main

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

type execWrapperCommandCall struct {
	cmd     []string
	oupfile *os.File
}

type testExecWrapper struct {
	calls   []execWrapperCommandCall
	command *testExecCommand
}

type StartResult struct {
	StartError error
	Pid        int
}

type testExecCommand struct {
	StartResults   chan StartResult
	WaitResults    chan error
	StartCallCount int
	WaitCallCount  int
}

type testTimerFactory struct {
	calls []time.Duration
	c     chan time.Time
}

func (e *testExecWrapper) Command(cmd []string, oupfile *os.File) ExecCommand {
	e.calls = append(e.calls, execWrapperCommandCall{cmd, oupfile})
	return e.command
}

func (c *testExecCommand) Start() (error, int) {
	c.StartCallCount += 1
	res := <-c.StartResults
	return res.StartError, res.Pid
}

func (c *testExecCommand) Wait() error {
	c.WaitCallCount += 1
	return <-c.WaitResults
}

func (f *testTimerFactory) NewTimer(timeout time.Duration) <-chan time.Time {
	f.calls = append(f.calls, timeout)
	return f.c
}

func TestRunnerJobReatemptsUntilFailure(t *testing.T) {
	execCommand := testExecCommand{
		StartResults:   make(chan StartResult),
		WaitResults:    make(chan error),
		StartCallCount: 0,
		WaitCallCount:  0,
	}
	execWrapper := testExecWrapper{command: &execCommand}
	timerFactory := testTimerFactory{}

	// un-buffered channels for testing
	resultChan := make(chan RunResult)
	jobDoneChan := make(chan bool)

	startRunner(Testee{
		cmd:        []string{"my-cmd", "--my-arg"},
		timeoutSec: 0,
	}, resultChan, jobDoneChan, &execWrapper, &timerFactory)

	for i := 0; i < 10; i++ {
		execCommand.StartResults <- StartResult{}
		execCommand.WaitResults <- nil // no error
		runResult := <-resultChan
		if runResult.result != RUNRES_OK {
			t.Errorf("Expected RUNRES_OK")
		}
		os.Remove(runResult.oupfile.Name())
	}
	execCommand.StartResults <- StartResult{}
	execCommand.WaitResults <- &exec.ExitError{
		ProcessState: &os.ProcessState{},
	}
	runResult := <-resultChan
	if runResult.result != RUNRES_FAIL {
		t.Errorf("Expected RUNRES_FAIL")
	}
	os.Remove(runResult.oupfile.Name())
}
