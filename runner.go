// Copyright (c) 2024 Marcel Heistermann

package main

import (
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"time"
)

type Testee struct {
	cmd        []string
	timeoutSec int
}

func StartRunner(testee Testee, results chan<- RunResult, done chan<- bool) RunnerJob {
	res := runnerJob{
		cmd:        testee.cmd,
		timeoutSec: testee.timeoutSec,
		resultChan: results,
		doneChan:   done,
		stopChan:   make(chan bool, 1),
	}
	go res.run()
	return &res
}

type RunnerJob interface {
	stop()
}

type runnerJob struct {
	cmd        []string
	timeoutSec int
	resultChan chan<- RunResult
	doneChan   chan<- bool
	stopChan   chan bool
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
		oupfile, tmpf_err := ioutil.TempFile(".", ".tofail_oup")
		if tmpf_err != nil {
			log.Fatal(tmpf_err)
		}
		execCmd := exec.Command(job.cmd[0], job.cmd[1:]...)
		execCmd.Stdout = oupfile
		execCmd.Stderr = oupfile
		var timeout <-chan time.Time
		if job.timeoutSec > 0 {
			timeout = time.NewTimer(time.Duration(job.timeoutSec) * time.Second).C
		} else {
			timeout = make(chan time.Time)
		}
		pid, cmdDone := make(chan int, 1), make(chan error)
		go func() {
			startErr := execCmd.Start()
			if startErr != nil {
				cmdDone <- startErr
				return
			}
			pid <- execCmd.Process.Pid
			cmdDone <- execCmd.Wait()
		}()
		select {
		case <-timeout:
			job.resultChan <- RunResult{result: RUNRES_TIMEOUT, oupfile: oupfile, timeoutPid: <-pid}
			<-cmdDone
			break loop
		case err := <-cmdDone:
			if err == nil {
				job.resultChan <- RunResult{result: RUNRES_OK, oupfile: oupfile, exitCode: 0}
			} else {
				switch e := err.(type) {
				case *exec.ExitError:
					oupfile.Seek(0, 0)
					job.resultChan <- RunResult{result: RUNRES_FAIL, oupfile: oupfile, exitCode: e.ExitCode()}
				default:
					job.resultChan <- RunResult{result: RUNRES_FAILED_EXECUTING, oupfile: oupfile}
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
