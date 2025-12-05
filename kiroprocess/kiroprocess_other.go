//go:build !windows

package kiroprocess

import (
	"os/exec"
	"strconv"
	"strings"
)

// getWindowsKiroProcesses 非 Windows 平台不支援
func getWindowsKiroProcesses() ([]ProcessInfo, error) {
	return nil, ErrUnsupportedPlatform
}

// killWindowsProcess 非 Windows 平台不支援
func killWindowsProcess(pid int) error {
	return ErrUnsupportedPlatform
}

func getDarwinKiroProcesses() ([]ProcessInfo, error) {
	cmd := exec.Command("pgrep", "-l", "Kiro")
	output, err := cmd.Output()
	if err != nil {
		return []ProcessInfo{}, nil
	}
	return parseUnixPgrep(string(output))
}

func getLinuxKiroProcesses() ([]ProcessInfo, error) {
	cmd := exec.Command("pgrep", "-l", "-i", "kiro")
	output, err := cmd.Output()
	if err != nil {
		return []ProcessInfo{}, nil
	}
	return parseUnixPgrep(string(output))
}

func parseUnixPgrep(output string) ([]ProcessInfo, error) {
	var processes []ProcessInfo
	lines := strings.Split(strings.TrimSpace(output), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) >= 2 {
			pid, err := strconv.Atoi(parts[0])
			if err == nil {
				processes = append(processes, ProcessInfo{
					PID:  pid,
					Name: parts[1],
				})
			}
		}
	}

	return processes, nil
}

// killUnixProcess 使用 kill 命令終止進程
func killUnixProcess(pid int) error {
	cmd := exec.Command("kill", "-9", strconv.Itoa(pid))
	return cmd.Run()
}
