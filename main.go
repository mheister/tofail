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
)

func main() {
	njobs := flag.Int("j", 1, "Number of concurrent jobs")
	flag.Parse()

	subject := os.Args[len(os.Args)-flag.NArg():]

	fmt.Println("CMD: ", os.Args)

	fmt.Println("njobs =", *njobs, "(ignored)")
	fmt.Println("command =", subject)

	oupfile, tmpf_err := ioutil.TempFile("", ".tofail_oup")
	if tmpf_err != nil {
		log.Fatal(tmpf_err)
	}
	defer os.Remove(oupfile.Name())
	// TODO Ctrl-C handler

	cmd := exec.Command(subject[0], subject[1:]...)
	cmd.Stdout = oupfile
	cmd.Stderr = oupfile
	err := cmd.Run()
	if err != nil {
		switch e := err.(type) {
		case *exec.Error:
			log.Fatal("failed executing:", err)
		case *exec.ExitError:
			fmt.Println("command exit rc =", e.ExitCode())

			oupfile.Seek(0, 0)
			oup, readerr := io.ReadAll(oupfile)
			if readerr != nil {
				log.Fatal(readerr)
			}
			fmt.Println(string(oup))
		default:
			panic(err)
		}
	}
}
