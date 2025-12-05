//go:build !windows

package machineid

import (
	"errors"
	"os/exec"
	"runtime"
	"strings"
)

func getWindowsMachineId() (string, error) {
	return "", errors.New("Windows-only function called on " + runtime.GOOS)
}

func getDarwinMachineId() (string, error) {
	cmd := exec.Command("ioreg", "-rd1", "-c", "IOPlatformExpertDevice")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "IOPlatformUUID") {
			parts := strings.Split(line, "=")
			if len(parts) == 2 {
				uuid := strings.TrimSpace(parts[1])
				uuid = strings.Trim(uuid, "\"")
				return strings.ToLower(uuid), nil
			}
		}
	}
	return "", errors.New("IOPlatformUUID not found")
}

func getLinuxMachineId() (string, error) {
	paths := []string{"/etc/machine-id", "/var/lib/dbus/machine-id"}
	for _, path := range paths {
		cmd := exec.Command("cat", path)
		output, err := cmd.Output()
		if err == nil {
			id := strings.TrimSpace(string(output))
			if id != "" {
				return strings.ToLower(id), nil
			}
		}
	}
	return "", errors.New("machine-id not found")
}
