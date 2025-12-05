package machineid

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"runtime"
)

// GetMachineId 取得系統的 Machine ID，經過 SHA-256 雜湊後回傳
func GetMachineId() (string, error) {
	rawId, err := GetRawMachineId()
	if err != nil {
		return "", err
	}
	return hashSHA256(rawId), nil
}

// GetRawMachineId 取得系統的原始 Machine ID（未雜湊）
func GetRawMachineId() (string, error) {
	switch runtime.GOOS {
	case "windows":
		return getWindowsMachineId()
	case "darwin":
		return getDarwinMachineId()
	case "linux":
		return getLinuxMachineId()
	default:
		return "", errors.New("unsupported platform: " + runtime.GOOS)
	}
}

func hashSHA256(data string) string {
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}
