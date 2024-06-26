// Copyright (c) 2024 Marcel Heistermann

package runner

import (
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/mheister/tofail/internal/execwrapper"
)

type execWrapperCommandCall struct {
	cmd []string
}

type testExecWrapper struct {
	calls   []execWrapperCommandCall
	command *testExecCommand
}

type testExecCommand struct {
	StartResultChan chan execwrapper.StartResult
	WaitResultChan  chan execwrapper.RunResult
	StartCallCount  int
	WaitCallCount   int
}

type testTimerFactory struct {
	calls []time.Duration
	c     chan time.Time
}

func (e *testExecWrapper) Command(cmd []string) execwrapper.ExecCommand {
	e.calls = append(e.calls, execWrapperCommandCall{cmd})
	return e.command
}

func (c *testExecCommand) StartWithTmpfile() execwrapper.StartResult {
	c.StartCallCount += 1
	return <-c.StartResultChan
}

func (c *testExecCommand) Wait() execwrapper.RunResult {
	c.WaitCallCount += 1
	return <-c.WaitResultChan
}

func (f *testTimerFactory) NewTimer(timeout time.Duration) <-chan time.Time {
	f.calls = append(f.calls, timeout)
	return f.c
}

type testError struct{}

func (testError) Error() string { return "<TESTERROR>" }

func TestRunnerJobAttemptsInvalidCommandAndFinishes(t *testing.T) {
	execCommand := testExecCommand{
		StartResultChan: make(chan execwrapper.StartResult),
		WaitResultChan:  make(chan execwrapper.RunResult),
		StartCallCount:  0,
		WaitCallCount:   0,
	}
	execWrapper := testExecWrapper{command: &execCommand}
	timerFactory := testTimerFactory{}

	// un-buffered channels for testing
	resultChan := make(chan RunResult)
	jobDoneChan := make(chan bool)

	startRunner(Testee{
		Cmd:        []string{"invalid-cmd"},
		TimeoutSec: 0,
	}, resultChan, jobDoneChan, &execWrapper, &timerFactory)

	oupfile := &os.File{}
	execCommand.StartResultChan <- execwrapper.StartResult{
		Error:   testError{},
		Pid:     0,
		Oupfile: oupfile,
	}
	// StartResults channel not buffered, so execWrapper.Command() was called here
	givenCmd := execWrapper.calls[len(execWrapper.calls)-1].cmd
	if !reflect.DeepEqual(givenCmd, []string{"invalid-cmd"}) {
		t.Errorf("Unexpected command given to execwrapper: %s", givenCmd)
	}
	runResult := <-resultChan
	if runResult.Result != RUNRES_FAILED_EXECUTING {
		t.Errorf("Expected RUNRES_FAILED_EXECUTING")
	}
	if runResult.Oupfile != oupfile {
		t.Errorf("Unexpected oupfile %p", oupfile)
	}
	<-jobDoneChan
}

func TestRunnerJobExecutesFailingCommandAndFinishes(t *testing.T) {
	execCommand := testExecCommand{
		StartResultChan: make(chan execwrapper.StartResult),
		WaitResultChan:  make(chan execwrapper.RunResult),
		StartCallCount:  0,
		WaitCallCount:   0,
	}
	execWrapper := testExecWrapper{command: &execCommand}
	timerFactory := testTimerFactory{}

	// un-buffered channels for testing
	resultChan := make(chan RunResult)
	jobDoneChan := make(chan bool)

	startRunner(Testee{
		Cmd:        []string{"my-cmd", "--my-arg"},
		TimeoutSec: 0,
	}, resultChan, jobDoneChan, &execWrapper, &timerFactory)

	oupfile := &os.File{}
	execCommand.StartResultChan <- execwrapper.StartResult{Oupfile: oupfile}
	// StartResults channel not buffered, so execWrapper.Command() was called here
	givenCmd := execWrapper.calls[len(execWrapper.calls)-1].cmd
	if !reflect.DeepEqual(givenCmd, []string{"my-cmd", "--my-arg"}) {
		t.Errorf("Unexpected command given to execwrapper: %s", givenCmd)
	}
	execCommand.WaitResultChan <- execwrapper.RunResult{
		ExitCode: 77,
		IoError:  nil,
	}
	runResult := <-resultChan
	if runResult.Result != RUNRES_FAIL {
		t.Errorf("Expected RUNRES_FAIL")
	}
	if runResult.ExitCode != 77 {
		t.Errorf("Expected exit code 77, was %d", runResult.ExitCode)
	}
	if runResult.Oupfile != oupfile {
		t.Errorf("Unexpected oupfile %p", oupfile)
	}
	<-jobDoneChan
}

func TestRunnerJobReatemptsUntilFailure(t *testing.T) {
	execCommand := testExecCommand{
		StartResultChan: make(chan execwrapper.StartResult),
		WaitResultChan:  make(chan execwrapper.RunResult),
		StartCallCount:  0,
		WaitCallCount:   0,
	}
	execWrapper := testExecWrapper{command: &execCommand}
	timerFactory := testTimerFactory{}

	// un-buffered channels for testing
	resultChan := make(chan RunResult)
	jobDoneChan := make(chan bool)

	startRunner(Testee{
		Cmd:        []string{"my-cmd", "--my-arg"},
		TimeoutSec: 0,
	}, resultChan, jobDoneChan, &execWrapper, &timerFactory)

	for i := 0; i < 10; i++ {
		oupfile := &os.File{}
		execCommand.StartResultChan <- execwrapper.StartResult{Oupfile: oupfile}
		execCommand.WaitResultChan <- execwrapper.RunResult{} // no error
		runResult := <-resultChan
		if runResult.Result != RUNRES_OK {
			t.Errorf("Expected RUNRES_OK")
		}
		if runResult.Oupfile != oupfile {
			t.Errorf("Unexpected oupfile %p", oupfile)
		}
	}
	execCommand.StartResultChan <- execwrapper.StartResult{}
	execCommand.WaitResultChan <- execwrapper.RunResult{
		ExitCode: 2,
		IoError:  nil,
	}
	runResult := <-resultChan
	if runResult.Result != RUNRES_FAIL {
		t.Errorf("Expected RUNRES_FAIL")
	}
	<-jobDoneChan
}

func TestRunnerStopsAfterIoErrorAndReportsFailure(t *testing.T) {
	execCommand := testExecCommand{
		StartResultChan: make(chan execwrapper.StartResult),
		WaitResultChan:  make(chan execwrapper.RunResult),
		StartCallCount:  0,
		WaitCallCount:   0,
	}
	execWrapper := testExecWrapper{command: &execCommand}
	timerFactory := testTimerFactory{}

	// un-buffered channels for testing
	resultChan := make(chan RunResult)
	jobDoneChan := make(chan bool)

	startRunner(Testee{
		Cmd:        []string{"my-cmd", "--my-arg"},
		TimeoutSec: 0,
	}, resultChan, jobDoneChan, &execWrapper, &timerFactory)

	oupfile := &os.File{}
	execCommand.StartResultChan <- execwrapper.StartResult{Oupfile: oupfile}
	execCommand.WaitResultChan <- execwrapper.RunResult{
		ExitCode: 0,
		IoError:  testError{},
	}
	runResult := <-resultChan
	if runResult.Result != RUNRES_FAIL {
		t.Errorf("Expected RUNRES_FAIL")
	}
	if runResult.Oupfile != oupfile {
		t.Errorf("Unexpected oupfile %p", oupfile)
	}
	<-jobDoneChan
}

func TestRunnerStopsAfterStopMethod(t *testing.T) {
	execCommand := testExecCommand{
		StartResultChan: make(chan execwrapper.StartResult),
		WaitResultChan:  make(chan execwrapper.RunResult),
		StartCallCount:  0,
		WaitCallCount:   0,
	}
	execWrapper := testExecWrapper{command: &execCommand}
	timerFactory := testTimerFactory{}

	resultChan := make(chan RunResult, 1)
	jobDoneChan := make(chan bool)

	runner := startRunner(Testee{
		Cmd:        []string{"my-cmd", "--my-arg"},
		TimeoutSec: 0,
	}, resultChan, jobDoneChan, &execWrapper, &timerFactory)

	execCommand.StartResultChan <- execwrapper.StartResult{}

	// between start and wait for exec we know that it receives stop before the next
	// run attempt
	runner.Stop()

	execCommand.WaitResultChan <- execwrapper.RunResult{}

	<-jobDoneChan
}

func TestRunnerStopsAfterTimeoutAndReportsTimeout(t *testing.T) {
	execCommand := testExecCommand{
		StartResultChan: make(chan execwrapper.StartResult),
		WaitResultChan:  make(chan execwrapper.RunResult),
		StartCallCount:  0,
		WaitCallCount:   0,
	}
	execWrapper := testExecWrapper{command: &execCommand}
	timerFactory := testTimerFactory{
		c: make(chan time.Time),
	}

	resultChan := make(chan RunResult, 1)
	jobDoneChan := make(chan bool)

	startRunner(Testee{
		Cmd:        []string{"my-cmd", "--my-arg"},
		TimeoutSec: 1,
	}, resultChan, jobDoneChan, &execWrapper, &timerFactory)

	oupfile := &os.File{}
	execCommand.StartResultChan <- execwrapper.StartResult{
		Error:   nil,
		Pid:     1337,
		Oupfile: oupfile,
	}
	if (len(timerFactory.calls) != 1) || timerFactory.calls[0] != time.Second {
		t.Errorf("Expected runner to get a timeout of 1s, got %s", timerFactory.calls)
	}
	timerFactory.c <- time.Now()
	execCommand.WaitResultChan <- execwrapper.RunResult{}
	runResult := <-resultChan
	if runResult.Result != RUNRES_TIMEOUT {
		t.Errorf("Expected RUNRES_TIMEOUT")
	}
	if runResult.TimeoutPid != 1337 {
		t.Errorf("Expected TimeoutPid 1337, was %d", runResult.TimeoutPid)
	}
	if runResult.Oupfile != oupfile {
		t.Errorf("Unexpected oupfile %p", oupfile)
	}
	<-jobDoneChan
}
