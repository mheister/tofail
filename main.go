// Copyright (c) 2024 Marcel Heistermann

package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
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

	resultChan := make(chan RunResult, 1024)
	jobDoneChan := make(chan bool, 1)

	sigint := make(chan os.Signal)
	signal.Notify(sigint, os.Interrupt)

	testee := Testee{
		cmd:        subject,
		timeoutSec: *timeoutSec,
	}

	jobs := make([]RunnerJob, *njobs)
	for i := 0; i < *njobs; i++ {
		jobs[i] = StartRunner(testee, resultChan, jobDoneChan)
	}

	stopping := false
	stopAll := func() bool {
		if stopping {
			return false
		}
		for _, c := range jobs {
			c.stop()
		}
		return true
	}

	nFailed := 0
	for jobsDone := 0; jobsDone < *njobs; {
		select {
		case <-jobDoneChan:
			jobsDone += 1
		case run := <-resultChan:
			switch run.result {
			case RUNRES_FAILED_EXECUTING:
				println("Failed to execute command!")
				if stopAll() && *njobs > 1 {
					println("Stopping all jobs.")
				}
				os.Remove(run.oupfile.Name())
			case RUNRES_OK:
				os.Remove(run.oupfile.Name())
			case RUNRES_FAIL:
				nFailed += 1
				stopAll := stopAll()
				fmt.Printf(
					"\n>> Failure #%d encountered! Exit code: %d. Output:\n",
					nFailed,
					run.exitCode)
				printOutput(run)
				fmt.Printf(
					"<< (#%d output end, exit code %d)\n",
					nFailed,
					run.exitCode)
				if stopAll && *njobs > 1 {
					println("Stopping all jobs.")
				}
				os.Remove(run.oupfile.Name())
			case RUNRES_TIMEOUT:
				stopAll := stopAll()
				fmt.Printf(
					"\nTimeout encountered! PID is %d, process is connected to output file '%s'\n",
					run.timeoutPid, run.oupfile.Name())
				println("The output file will not be deleted automatically.")
				if stopAll && *njobs > 1 {
					println("Stopping all jobs.")
				}
			}
		case <-sigint:
			stopAll()
			println(">> Ctrl-C signal, stopping all jobs.")
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
