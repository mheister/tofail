// Copyright (c) 2024 Marcel Heistermann

package execwrapper

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

type RunResult struct {
	ExitCode int
	IoError error
}

type ExecCommand interface {
	StartWithTmpfile() StartResult
	Wait() RunResult
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

func (cmd *execCommand) Wait() RunResult {
	waitErr := cmd.execCmd.Wait()
	switch e := waitErr.(type) {
	case *exec.ExitError:
		return RunResult{e.ExitCode(), nil}
	default:
		return RunResult{-1, e}
	}
}
