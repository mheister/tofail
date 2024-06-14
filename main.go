// Copyright (c) 2024 Marcel Heistermann

package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"time"
)

func main() {
	njobs := flag.Int("j", 1, "Number of concurrent jobs")
	timeoutSec := flag.Int(
		"timeout",
		0,
		"Give a number of seconds after which tofail should view a command as "+
			"failed (stuck) and print its PID")
	flag.Parse()

	if *njobs < 1 {
		log.Fatal("Error: Please specify >= 1 jobs")
	}
	if flag.NArg() < 1 {
		log.Fatal("Error: Missing command")
	}
	subject := os.Args[len(os.Args)-flag.NArg():]

	fmt.Println("Executing command ", subject, " repeatedly in ", *njobs, " job(s)")

	runs := make(chan RunResult, 1024)
	done := make(chan bool, 1)

	sigint := make(chan os.Signal)
	signal.Notify(sigint, os.Interrupt)

	var quitChans []chan bool
	for i := 0; i < *njobs; i++ {
		quitChans = append(quitChans, make(chan bool, 1))
		job := RunnerJob{
			cmd:        subject,
			timeoutSec: *timeoutSec,
		}
		go job.run(runs, quitChans[len(quitChans)-1], done)
	}

	quitting := false
	quitAll := func() bool {
		if quitting {
			return false
		}
		quitting = true
		for _, c := range quitChans {
			c <- true
			close(c)
		}
		return true
	}

	nFailed := 0
	for jobsDone := 0; jobsDone < *njobs; {
		select {
		case <-done:
			jobsDone += 1
		case run := <-runs:
			switch run.result {
			case RUNRES_FAILED_EXECUTING:
				println("Failed to execute command!")
				if quitAll() && *njobs > 1 {
					println("Quitting all jobs.")
				}
				os.Remove(run.oupfile.Name())
			case RUNRES_OK:
				os.Remove(run.oupfile.Name())
			case RUNRES_FAIL:
				nFailed += 1
				quitAll := quitAll()
				fmt.Printf(
					"\n>> Failure #%d encountered! Exit code: %d. Output:\n",
					nFailed,
					run.exitCode)
				printOutput(run)
				fmt.Printf(
					"<< (#%d output end, exit code %d)\n",
					nFailed,
					run.exitCode)
				if quitAll && *njobs > 1 {
					println("Quitting all jobs.")
				}
				os.Remove(run.oupfile.Name())
			case RUNRES_TIMEOUT:
				quitAll := quitAll()
				fmt.Printf(
					"\nTimeout encountered! PID is %d, process is connected to output file '%s'\n",
					run.timeoutPid, run.oupfile.Name())
				println("The output file will not be deleted automatically.")
				if quitAll && *njobs > 1 {
					println("Quitting all jobs.")
				}
			}
		case <-sigint:
			quitAll()
			println(">> Ctrl-C signal, quitting all jobs.")
		}
	}
}

func printOutput(run RunResult) {
	oup, readerr := io.ReadAll(run.oupfile)
	if readerr != nil {
		log.Fatal(readerr)
	}
	fmt.Println(string(oup))
}

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
