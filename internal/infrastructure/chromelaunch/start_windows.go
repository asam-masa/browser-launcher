//go:build windows

package chromelaunch

import (
	"fmt"
	"os"

	application "github.com/asam-masa/browser-launcher/internal/application/chromelaunch"
)

type process interface {
	PID() int
	Release() error
}

type processStarter interface {
	Start(executablePath string, arguments ...string) (process, error)
}

type nativeProcessStarter struct{}

type nativeProcess struct {
	process *os.Process
}

func (Provider) Start(command application.Command) (application.Result, error) {
	return start(nativeProcessStarter{}, command)
}

func start(starter processStarter, command application.Command) (application.Result, error) {
	startedProcess, err := starter.Start(command.ExecutablePath(), command.Arguments()...)
	if err != nil {
		return application.Result{}, fmt.Errorf("start process: %w", err)
	}

	result := application.Result{ProcessID: startedProcess.PID()}
	if err := startedProcess.Release(); err != nil {
		return result, fmt.Errorf("release process: %w", err)
	}
	return result, nil
}

func (nativeProcessStarter) Start(executablePath string, arguments ...string) (process, error) {
	argv := make([]string, 1, len(arguments)+1)
	argv[0] = executablePath
	argv = append(argv, arguments...)
	startedProcess, err := os.StartProcess(executablePath, argv, processAttributes())
	if err != nil {
		return nil, err
	}
	return nativeProcess{process: startedProcess}, nil
}

func processAttributes() *os.ProcAttr {
	return &os.ProcAttr{
		Files: []*os.File{nil, nil, nil},
	}
}

func (p nativeProcess) PID() int {
	return p.process.Pid
}

func (p nativeProcess) Release() error {
	return p.process.Release()
}
