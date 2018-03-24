// +build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
)

func openUrl(url string) {
	rundll := filepath.Join(os.Getenv("SYSTEMROOT"), "system32", "rundll32.exe")
	exec.Command(rundll, "url.dll,FileProtocolHandler", url).Run()
}
