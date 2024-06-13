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
)

func main() {
	njobs := flag.Int("j", 1, "Number of concurrent jobs")
	flag.Parse()

	if *njobs < 1 {
		log.Fatal("Error: Please specify >= 1 jobs")
	}
	if flag.NArg() < 1 {
		log.Fatal("Error: Missing command")
	}
	subject := os.Args[len(os.Args)-flag.NArg():]

	fmt.Println("Executing command ", subject, " repeatedly in ", *njobs, " job(s)")

	runs := make(chan Run, 1024)
	done := make(chan bool, 1)

	sigint := make(chan os.Signal)
	signal.Notify(sigint, os.Interrupt)

	var quitChans []chan bool
	for i := 0; i < *njobs; i++ {
		quitChans = append(quitChans, make(chan bool))
		go run(subject, runs, quitChans[len(quitChans)-1], done)
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
				if quitAll() {
					println(">> Failed to execute command, quitting all jobs!")
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
				if quitAll {
					println("Quitting all jobs.")
				}
				os.Remove(run.oupfile.Name())
			}
		case <-sigint:
			quitAll()
			println(">> Ctrl-C signal, quitting all jobs.")
		}
	}
}

func printOutput(run Run) {
	oup, readerr := io.ReadAll(run.oupfile)
	if readerr != nil {
		log.Fatal(readerr)
	}
	fmt.Println(string(oup))
}

type RunResult string

const (
	RUNRES_OK               RunResult = "OK"
	RUNRES_FAILED_EXECUTING RunResult = "FAILED_EXECUTING"
	RUNRES_FAIL             RunResult = "FAIL"
)

type Run struct {
	result   RunResult
	oupfile  *os.File
	exitCode int
}

func run(cmd []string, runs chan<- Run, quit <-chan bool, done chan<- bool) {
	for {
		select {
		case <-quit:
			done <- true
			return
		default:
		}
		oupfile, tmpf_err := ioutil.TempFile("", ".tofail_oup")
		if tmpf_err != nil {
			log.Fatal(tmpf_err)
		}
		exec_cmd := exec.Command(cmd[0], cmd[1:]...)
		exec_cmd.Stdout = oupfile
		exec_cmd.Stderr = oupfile
		err := exec_cmd.Run()
		if err == nil {
			runs <- Run{result: RUNRES_OK, oupfile: oupfile, exitCode: 0}
		} else {
			switch e := err.(type) {
			case *exec.Error:
				runs <- Run{result: RUNRES_FAILED_EXECUTING, oupfile: oupfile}
			case *exec.ExitError:
				oupfile.Seek(0, 0)
				runs <- Run{result: RUNRES_FAIL, oupfile: oupfile, exitCode: e.ExitCode()}
			default:
				runs <- Run{result: RUNRES_FAILED_EXECUTING, oupfile: oupfile}
			}
		}
	}
}
