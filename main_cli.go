//go:build cli

package main

import (
	"fmt"
	"kiro-manager/awssso"
	"kiro-manager/backup"
	"kiro-manager/kiropath"
	"kiro-manager/machineid"
)

func main() {
	// Machine ID 示範
	rawId, err := machineid.GetRawMachineId()
	if err != nil {
		fmt.Printf("Error getting raw machine id: %v\n", err)
		return
	}
	fmt.Printf("Raw Machine ID: %s\n", rawId)

	hashedId, err := machineid.GetMachineId()
	if err != nil {
		fmt.Printf("Error getting hashed machine id: %v\n", err)
		return
	}
	fmt.Printf("Hashed Machine ID (SHA-256): %s\n", hashedId)

	fmt.Println()

	// Kiro 路徑偵測示範
	fmt.Println("=== Kiro Path Detection ===")

	kiroHome, err := kiropath.GetKiroHomePath()
	if err != nil {
		fmt.Printf("Error getting Kiro home path: %v\n", err)
	} else {
		exists := ""
		if kiropath.KiroHomeExists() {
			exists = " [EXISTS]"
		}
		fmt.Printf("Kiro Home (~/.kiro): %s%s\n", kiroHome, exists)
	}

	kiroConfig, err := kiropath.GetKiroConfigPath()
	if err != nil {
		fmt.Printf("Error getting Kiro config path: %v\n", err)
	} else {
		exists := ""
		if kiropath.KiroConfigExists() {
			exists = " [EXISTS]"
		}
		fmt.Printf("Kiro Config: %s%s\n", kiroConfig, exists)
	}

	kiroInstall, err := kiropath.GetKiroInstallPath()
	if err != nil {
		fmt.Printf("Kiro Install: %v\n", err)
	} else {
		fmt.Printf("Kiro Install: %s [INSTALLED]\n", kiroInstall)
	}

	fmt.Printf("Kiro Installed: %v\n", kiropath.IsKiroInstalled())

	fmt.Println()

	// AWS 路徑偵測示範
	fmt.Println("=== AWS Path Detection ===")

	awsConfig, err := kiropath.GetAWSConfigPath()
	if err != nil {
		fmt.Printf("Error getting AWS config path: %v\n", err)
	} else {
		exists := ""
		if kiropath.AWSConfigExists() {
			exists = " [EXISTS]"
		}
		fmt.Printf("AWS Config (~/.aws): %s%s\n", awsConfig, exists)
	}

	fmt.Println()

	// AWS SSO Cache 示範
	fmt.Println("=== AWS SSO Cache ===")

	ssoCachePath, err := awssso.GetSSOCachePath()
	if err != nil {
		fmt.Printf("Error getting SSO cache path: %v\n", err)
	} else {
		exists := ""
		if awssso.SSOCacheExists() {
			exists = " [EXISTS]"
		}
		fmt.Printf("SSO Cache Path: %s%s\n", ssoCachePath, exists)
	}

	// 列出快取檔案
	cacheFiles, err := awssso.ListCacheFiles()
	if err != nil {
		fmt.Printf("Error listing cache files: %v\n", err)
	} else {
		fmt.Printf("Cache Files (%d):\n", len(cacheFiles))
		for _, file := range cacheFiles {
			fmt.Printf("  - %s\n", file)
		}
	}

	// 讀取 Kiro Auth Token
	token, err := awssso.ReadKiroAuthToken()
	if err != nil {
		fmt.Printf("Kiro Auth Token: %v\n", err)
	} else {
		fmt.Println("Kiro Auth Token:")
		fmt.Printf("  Provider: %s\n", token.Provider)
		fmt.Printf("  AuthMethod: %s\n", token.AuthMethod)
		fmt.Printf("  ExpiresAt: %s\n", token.ExpiresAt)
		if token.AccessToken != "" {
			fmt.Printf("  AccessToken: %s...\n", token.AccessToken[:min(20, len(token.AccessToken))])
		}
	}

	fmt.Println()

	// Backup 模組示範
	fmt.Println("=== Backup Module ===")

	backupRoot, err := backup.GetBackupRootPath()
	if err != nil {
		fmt.Printf("Error getting backup root path: %v\n", err)
	} else {
		fmt.Printf("Backup Root: %s\n", backupRoot)
	}

	// 列出所有備份
	backups, err := backup.ListBackups()
	if err != nil {
		fmt.Printf("Error listing backups: %v\n", err)
	} else {
		fmt.Printf("Backups (%d):\n", len(backups))
		for _, b := range backups {
			fmt.Printf("  - %s (Token: %v, MachineID: %v, Time: %s)\n",
				b.Name, b.HasToken, b.HasMachineID, b.BackupTime.Format("2006-01-02 15:04:05"))
		}
	}
}
