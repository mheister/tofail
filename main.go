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
