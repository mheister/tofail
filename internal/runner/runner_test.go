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

	execCommand.StartResultChan <- execwrapper.StartResult{
		Error:   testError{},
		Pid:     0,
		Oupfile: &os.File{},
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

	execCommand.StartResultChan <- execwrapper.StartResult{}
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
		execCommand.StartResultChan <- execwrapper.StartResult{}
		execCommand.WaitResultChan <- execwrapper.RunResult{} // no error
		runResult := <-resultChan
		if runResult.Result != RUNRES_OK {
			t.Errorf("Expected RUNRES_OK")
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

	execCommand.StartResultChan <- execwrapper.StartResult{}
	execCommand.WaitResultChan <- execwrapper.RunResult{
		ExitCode: 0,
		IoError:  testError{},
	}
	runResult := <-resultChan
	if runResult.Result != RUNRES_FAIL {
		t.Errorf("Expected RUNRES_FAIL")
	}
	<-jobDoneChan
}
