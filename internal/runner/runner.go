// Copyright (c) 2024 Marcel Heistermann

package runner

import (
	"log"
	"os"
	"time"

	"github.com/mheister/tofail/internal/execwrapper"
	"github.com/mheister/tofail/internal/timeout"
)

type Testee struct {
	Cmd        []string
	TimeoutSec int
}

func StartJob(testee Testee, results chan<- RunResult, done chan<- bool) Job {
	return startRunner(
		testee,
		results,
		done,
		execwrapper.GetOsExecWrapper(),
		timeout.GetDefaultTimerFactory())
}

func startRunner(
	testee Testee,
	results chan<- RunResult,
	done chan<- bool,
	execWrapper execwrapper.ExecWrapper,
	timerFactory timeout.TimerFactory,
) Job {
	res := runnerJob{
		cmd:          testee.Cmd,
		timeoutSec:   testee.TimeoutSec,
		resultChan:   results,
		doneChan:     done,
		stopChan:     make(chan bool, 1),
		execWrapper:  execWrapper,
		timerFactory: timerFactory,
	}
	go res.run()
	return &res
}

type Job interface {
	Stop()
}

type runnerJob struct {
	cmd          []string
	timeoutSec   int
	resultChan   chan<- RunResult
	doneChan     chan<- bool
	stopChan     chan bool
	execWrapper  execwrapper.ExecWrapper
	timerFactory timeout.TimerFactory
}

type RunResultType string

const (
	RUNRES_OK               RunResultType = "OK"
	RUNRES_FAILED_EXECUTING RunResultType = "FAILED_EXECUTING"
	RUNRES_FAIL             RunResultType = "FAIL"
	RUNRES_TIMEOUT          RunResultType = "TIMEOUT"
)

type RunResult struct {
	Result     RunResultType
	Oupfile    *os.File
	ExitCode   int
	TimeoutPid int
}

func (job *runnerJob) run() {
loop:
	for {
		select {
		case <-job.stopChan:
			break loop
		default:
		}
		execCmd := job.execWrapper.Command(job.cmd)
		var timeout <-chan time.Time
		if job.timeoutSec > 0 {
			timeout = job.timerFactory.NewTimer(time.Duration(job.timeoutSec) * time.Second)
		} else {
			timeout = make(chan time.Time)
		}
		startChan := make(chan execwrapper.StartResult, 1)
		cmdDoneChan := make(chan execwrapper.RunResult)
		go func() {
			startRes := execCmd.StartWithTmpfile()
			startChan <- startRes
			if startRes.Error != nil {
				return
			}
			cmdDoneChan <- execCmd.Wait()
		}()
		startRes := <-startChan
		if startRes.Error != nil {
			job.resultChan <- RunResult{Result: RUNRES_FAILED_EXECUTING, Oupfile: startRes.Oupfile}
			break loop
		}
		select {
		case <-timeout:
			job.resultChan <- RunResult{
				Result: RUNRES_TIMEOUT, Oupfile: startRes.Oupfile, TimeoutPid: startRes.Pid}
			<-cmdDoneChan
			break loop
		case result := <-cmdDoneChan:
			if result.ExitCode == 0 && result.IoError == nil {
				job.resultChan <- RunResult{Result: RUNRES_OK, Oupfile: startRes.Oupfile, ExitCode: 0}
				continue
			}
			if result.IoError != nil {
				log.Println("Warning: Command", job.cmd, "likely enountered I/O error")
			}
			job.resultChan <- RunResult{
				Result: RUNRES_FAIL, Oupfile: startRes.Oupfile, ExitCode: result.ExitCode}
			break loop
		}
	}
	job.doneChan <- true
}

func (job *runnerJob) Stop() {
	job.stopChan <- true
	close(job.stopChan)
}
