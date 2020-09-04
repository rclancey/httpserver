package httpserver

import (
	"net"
	"os"
	"strconv"
	"syscall"

	"github.com/pkg/errors"
)


func checkRunning(cfg *ServerConfig) error {
	err := checkRunningPidfile(cfg.PidFile)
	if err != nil {
		return errors.WithStack(err)
	}
	err = checkRunningPort(cfg.Bind.Port)
	if err != nil {
		return errors.WithStack(err)
	}
	if cfg.Bind.SSL.Enabled() {
		err = checkRunningPort(cfg.Bind.SSL.Port)
		if err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
}

func checkRunningPidfile(fn string) error {
	pidF, err := os.Open(fn)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return errors.Wrap(err, "can't open pid file " + fn)
	}
	defer pidF.Close()
	pidData := make([]byte, 256)
	n, err := pidF.Read(pidData)
	if err != nil {
		return errors.Wrap(err, "can't read pid file " + fn)
	}
	if n == 0 {
		return nil
	}
	pid, err := strconv.ParseInt(string(pidData[:n]), 10, 32)
	if err != nil {
		return errors.Wrap(err, "can't decode pid " + string(pidData[:n]))
	}
	proc, err := os.FindProcess(int(pid))
	if err != nil {
		return errors.Wrapf(err, "can't find process %d", pid)
	}
	err = proc.Signal(syscall.Signal(0))
	if err == nil {
		return errors.Errorf("%s already running at PID %d", os.Args[0], pid)
	}
	return nil
}

func checkRunningPort(port int) error {
	ln, err := net.Listen("tcp", ":" + strconv.Itoa(port))
	if err != nil {
		return errors.Errorf("port %d is already in use", port)
	}
	ln.Close()
	return nil
}

func writePidfile(cfg *ServerConfig) error {
	pidF, err := os.Create(cfg.PidFile)
	if err != nil {
		return errors.Wrap(err, "can't create pid file " + cfg.PidFile)
	}
	_, err = pidF.Write([]byte(strconv.Itoa(os.Getpid())))
	if err != nil {
		return errors.Wrap(err, "can't write pid file " + cfg.PidFile)
	}
	return errors.Wrap(pidF.Close(), "can't close pid file " + cfg.PidFile)
}

func removePidfile(cfg *ServerConfig) error {
	return errors.Wrap(os.Remove(cfg.PidFile), "can't remove pid file " + cfg.PidFile)
}
