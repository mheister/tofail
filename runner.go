// Copyright (c) 2024 Marcel Heistermann

package main

import (
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"time"
)

type RunnerJob struct {
	cmd        []string
	timeoutSec int
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

func (job RunnerJob) run(results chan<- RunResult, quit <-chan bool, done chan<- bool) {
loop:
	for {
		select {
		case <-quit:
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
			results <- RunResult{result: RUNRES_TIMEOUT, oupfile: oupfile, timeoutPid: <-pid}
			<-cmdDone
			break loop
		case err := <-cmdDone:
			if err == nil {
				results <- RunResult{result: RUNRES_OK, oupfile: oupfile, exitCode: 0}
			} else {
				switch e := err.(type) {
				case *exec.ExitError:
					oupfile.Seek(0, 0)
					results <- RunResult{result: RUNRES_FAIL, oupfile: oupfile, exitCode: e.ExitCode()}
				default:
					results <- RunResult{result: RUNRES_FAILED_EXECUTING, oupfile: oupfile}
				}
				break loop
			}
		}
	}
	done <- true
}
