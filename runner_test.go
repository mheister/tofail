// Copyright (c) 2024 Marcel Heistermann

package main

import (
	"os"
	"os/exec"
	"reflect"
	"testing"
	"time"
)

type execWrapperCommandCall struct {
	cmd []string
}

type testExecWrapper struct {
	calls   []execWrapperCommandCall
	command *testExecCommand
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

func (e *testExecWrapper) Command(cmd []string) ExecCommand {
	e.calls = append(e.calls, execWrapperCommandCall{cmd})
	return e.command
}

func (c *testExecCommand) StartWithTmpfile() StartResult {
	c.StartCallCount += 1
	return <-c.StartResults
}

func (c *testExecCommand) Wait() error {
	c.WaitCallCount += 1
	return <-c.WaitResults
}

func (f *testTimerFactory) NewTimer(timeout time.Duration) <-chan time.Time {
	f.calls = append(f.calls, timeout)
	return f.c
}

type testLaunchError struct{}

func (testLaunchError) Error() string { return "Could not launch" }

func TestRunnerJobAttemptsInvalidCommandAndFinishes(t *testing.T) {
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
		cmd:        []string{"invalid-cmd"},
		timeoutSec: 0,
	}, resultChan, jobDoneChan, &execWrapper, &timerFactory)

	execCommand.StartResults <- StartResult{
		Error:   testLaunchError{},
		Pid:     0,
		Oupfile: &os.File{},
	}
	// StartResults channel not buffered, so execWrapper.Command() was called here
	givenCmd := execWrapper.calls[len(execWrapper.calls)-1].cmd
	if !reflect.DeepEqual(givenCmd, []string{"invalid-cmd"}) {
		t.Errorf("Unexpected command given to execwrapper: %s", givenCmd)
	}
	runResult := <-resultChan
	if runResult.result != RUNRES_FAILED_EXECUTING {
		t.Errorf("Expected RUNRES_FAILED_EXECUTING")
	}
	<-jobDoneChan
}

func TestRunnerJobExecutesFailingCommandAndFinishes(t *testing.T) {
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

	execCommand.StartResults <- StartResult{}
	// StartResults channel not buffered, so execWrapper.Command() was called here
	givenCmd := execWrapper.calls[len(execWrapper.calls)-1].cmd
	if !reflect.DeepEqual(givenCmd, []string{"my-cmd", "--my-arg"}) {
		t.Errorf("Unexpected command given to execwrapper: %s", givenCmd)
	}
	execCommand.WaitResults <- &exec.ExitError{
		ProcessState: &os.ProcessState{},
	}
	runResult := <-resultChan
	if runResult.result != RUNRES_FAIL {
		t.Errorf("Expected RUNRES_FAIL")
	}
	<-jobDoneChan
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
	}
	execCommand.StartResults <- StartResult{}
	execCommand.WaitResults <- &exec.ExitError{
		ProcessState: &os.ProcessState{},
	}
	runResult := <-resultChan
	if runResult.result != RUNRES_FAIL {
		t.Errorf("Expected RUNRES_FAIL")
	}
}
