package handlers

import (
	"bufio"
	"os"
	"os/exec"
	"regexp"
	"strings"

	client "github.com/rancher/go-rancher/v2"
)

var (
	RegExMachineDirEnv      = regexp.MustCompile("^" + machineDirEnvKey + ".*")
	RegExMachinePluginToken = regexp.MustCompile("^" + "MACHINE_PLUGIN_TOKEN=" + ".*")
	RegExMachineDriverName  = regexp.MustCompile("^" + "MACHINE_PLUGIN_DRIVER_NAME=" + ".*")
)

func deleteMachine(machineDir string, machine *client.Machine) error {
	command := buildCommand(machineDir, []string{"rm", "-f", machine.Name})
	err := command.Start()
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
	command := buildCommand(machineDir, []string{"ls", "-f", "{{.State}}", machine.Name})
	output, err := command.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func machineExists(machineDir string, name string) (bool, error) {
	command := buildCommand(machineDir, []string{"ls", "-q"})
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

func buildCommand(machineDir string, cmdArgs []string) *exec.Cmd {
	command := exec.Command(machineCmd, cmdArgs...)
	env := initEnviron(machineDir)
	command.Env = env
	return command
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
