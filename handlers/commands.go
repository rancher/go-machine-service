package handlers

import (
	"bufio"
	"os"
	"os/exec"
	"os/user"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"github.com/pkg/errors"
	client "github.com/rancher/go-rancher/v2"
)

var (
	RegExMachineDirEnv      = regexp.MustCompile("^" + machineDirEnvKey + ".*")
	RegExMachinePluginToken = regexp.MustCompile("^" + "MACHINE_PLUGIN_TOKEN=" + ".*")
	RegExMachineDriverName  = regexp.MustCompile("^" + "MACHINE_PLUGIN_DRIVER_NAME=" + ".*")
)

func deleteMachine(machineDir string, machine *client.Machine) error {
	command, err := buildCommand(machineDir, []string{"rm", "-f", machine.Name})
	if err != nil {
		return err
	}
	err = command.Start()
	if err != nil {
		return err
	}

	err = command.Wait()
	if err != nil {
		return err
	}

	return nil
}

func getState(machineDir string, machine *client.Machine) (string, error) {
	command, err := buildCommand(machineDir, []string{"ls", "-f", "{{.State}}", machine.Name})
	if err != nil {
		return "", err
	}
	output, err := command.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func machineExists(machineDir string, name string) (bool, error) {
	command, err := buildCommand(machineDir, []string{"ls", "-q"})
	if err != nil {
		return false, err
	}
	r, err := command.StdoutPipe()
	if err != nil {
		return false, err
	}

	err = command.Start()
	if err != nil {
		return false, err
	}

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		foundName := scanner.Text()
		if foundName == name {
			return true, nil
		}
	}
	if err = scanner.Err(); err != nil {
		return false, err
	}

	err = command.Wait()
	if err != nil {
		return false, err
	}

	return false, nil
}

func buildCommand(machineDir string, cmdArgs []string) (*exec.Cmd, error) {
	if os.Getenv("DISABLE_DRIVER_JAIL") == "true" {
		command := exec.Command(machineCmd, cmdArgs...)
		env := initEnviron(machineDir)
		command.Env = env
		return command, nil
	}

	cred, err := getUserCred()
	if err != nil {
		return nil, errors.WithMessage(err, "get user cred error")
	}

	command := exec.Command(machineCmd, cmdArgs...)
	command.SysProcAttr = &syscall.SysProcAttr{}
	command.SysProcAttr.Credential = cred
	command.SysProcAttr.Chroot = machineDir
	command.Env = []string{
		machineDirEnvKey + machineDir,
		"PATH=/usr/bin:/usr/local/bin",
	}
	return command, nil

}

func initEnviron(machineDir string) []string {
	env := os.Environ()
	found := false
	for idx, ev := range env {
		if RegExMachineDirEnv.MatchString(ev) {
			env[idx] = machineDirEnvKey + machineDir
			found = true
		}
		if RegExMachinePluginToken.MatchString(ev) {
			env[idx] = ""
		}
		if RegExMachineDriverName.MatchString(ev) {
			env[idx] = ""
		}
	}
	if !found {
		env = append(env, machineDirEnvKey+machineDir)
	}
	return env
}

// getUserCred looks up the user and provides it in syscall.Credential
func getUserCred() (*syscall.Credential, error) {
	u, err := user.Current()
	if err != nil {
		uID := os.Getuid()
		u, err = user.LookupId(strconv.Itoa(uID))
		if err != nil {
			return nil, err
		}
	}

	i, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		return nil, err
	}
	uid := uint32(i)

	i, err = strconv.ParseUint(u.Gid, 10, 32)
	if err != nil {
		return nil, err
	}
	gid := uint32(i)

	return &syscall.Credential{Uid: uid, Gid: gid}, nil
}
