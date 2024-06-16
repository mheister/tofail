// Copyright (c) 2024 Marcel Heistermann

package main

import (
	"io/ioutil"
	"log"
	"os"
	"os/exec"
)

type ExecWrapper interface {
	Command(cmd []string) ExecCommand
}

type StartResult struct {
	Error error
	Pid int
	Oupfile *os.File
}

type ExecCommand interface {
	StartWithTmpfile() StartResult
	Wait() error
}

func GetOsExecWrapper() ExecWrapper {
	return &osExecWrapper{}
}

type osExecWrapper struct{}

type execCommand struct {
	execCmd *exec.Cmd
}

func (*osExecWrapper) Command(cmd []string) ExecCommand {
	execCmd := exec.Command(cmd[0], cmd[1:]...)
	return &execCommand{execCmd: execCmd}
}

func (cmd *execCommand) StartWithTmpfile() StartResult {
	oupfile, tmpf_err := ioutil.TempFile(".", ".tofail_oup")
	if tmpf_err != nil {
		log.Fatal(tmpf_err)
	}
	cmd.execCmd.Stdout = oupfile
	cmd.execCmd.Stderr = oupfile
	startErr := cmd.execCmd.Start()
	if startErr != nil {
		return StartResult{startErr, 0, oupfile}
	} else {
		return StartResult{nil, cmd.execCmd.Process.Pid, oupfile}
	}
}

func (cmd *execCommand) Wait() error {
	return cmd.execCmd.Wait()
}
