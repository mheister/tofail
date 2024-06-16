// Copyright (c) 2024 Marcel Heistermann

package main

import (
	"os"
	"os/exec"
	"time"
)

type Testee struct {
	cmd        []string
	timeoutSec int
}

func StartRunner(testee Testee, results chan<- RunResult, done chan<- bool) RunnerJob {
	return startRunner(testee, results, done, GetOsExecWrapper(), GetDefaultTimerFactory())
}

func startRunner(
	testee Testee,
	results chan<- RunResult,
	done chan<- bool,
	execWrapper ExecWrapper,
	timerFactory TimerFactory,
) RunnerJob {
	res := runnerJob{
		cmd:          testee.cmd,
		timeoutSec:   testee.timeoutSec,
		resultChan:   results,
		doneChan:     done,
		stopChan:     make(chan bool, 1),
		execWrapper:  execWrapper,
		timerFactory: timerFactory,
	}
	go res.run()
	return &res
}

type RunnerJob interface {
	stop()
}

type runnerJob struct {
	cmd          []string
	timeoutSec   int
	resultChan   chan<- RunResult
	doneChan     chan<- bool
	stopChan     chan bool
	execWrapper  ExecWrapper
	timerFactory TimerFactory
}

type RunResultType string

const (
	RUNRES_OK               RunResultType = "OK"
	RUNRES_FAILED_EXECUTING RunResultType = "FAILED_EXECUTING"
	RUNRES_FAIL             RunResultType = "FAIL"
	RUNRES_TIMEOUT          RunResultType = "TIMEOUT"
)

type RunResult struct {
	result     RunResultType
	oupfile    *os.File
	exitCode   int
	timeoutPid int
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
		startChan, cmdDoneChan := make(chan StartResult, 1), make(chan error)
		go func() {
			startRes:= execCmd.StartWithTmpfile()
			startChan <- startRes
			if startRes.Error != nil {
				return
			}
			cmdDoneChan <- execCmd.Wait()
		}()
		startRes := <- startChan
		if startRes.Error != nil {
			job.resultChan <- RunResult{result: RUNRES_FAILED_EXECUTING, oupfile: startRes.Oupfile}
			break loop
		}
		select {
		case <-timeout:
			job.resultChan <- RunResult{
				result: RUNRES_TIMEOUT, oupfile: startRes.Oupfile, timeoutPid: startRes.Pid}
			<-cmdDoneChan
			break loop
		case err := <-cmdDoneChan:
			if err == nil {
				job.resultChan <- RunResult{result: RUNRES_OK, oupfile: startRes.Oupfile, exitCode: 0}
			} else {
				switch e := err.(type) {
				case *exec.ExitError:
					startRes.Oupfile.Seek(0, 0)
					job.resultChan <- RunResult{
						result: RUNRES_FAIL, oupfile: startRes.Oupfile, exitCode: e.ExitCode()}
				default:
					job.resultChan <- RunResult{
						result: RUNRES_FAILED_EXECUTING, oupfile: startRes.Oupfile}
				}
				break loop
			}
		}
	}
	job.doneChan <- true
}

func (job *runnerJob) stop() {
	job.stopChan <- true
	close(job.stopChan)
}
