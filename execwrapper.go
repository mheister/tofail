// Copyright (c) 2024 Marcel Heistermann

package main

import (
	"os"
	"os/exec"
)

type ExecWrapper interface {
	Command(cmd []string, oupfile *os.File) ExecCommand
}

type ExecCommand interface {
	Start() (error, int) // error or PID
	Wait() error
}

func GetOsExecWrapper() ExecWrapper {
	return &osExecWrapper{}
}

type osExecWrapper struct{}

type execCommand struct {
	execCmd *exec.Cmd
}

func (*osExecWrapper) Command(cmd []string, oupfile *os.File) ExecCommand {
	execCmd := exec.Command(cmd[0], cmd[1:]...)
	execCmd.Stdout = oupfile
	execCmd.Stderr = oupfile
	return &execCommand{execCmd: execCmd}
}

func (cmd *execCommand) Start() (error, int) {
	startErr := cmd.execCmd.Start()
	if startErr != nil {
		return startErr, 0
	} else {
		return nil, cmd.execCmd.Process.Pid
	}
}

func (cmd *execCommand) Wait() error {
	return cmd.execCmd.Wait()
}
