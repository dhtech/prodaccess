// +build linux

package main

import (
	"os/exec"
)

func openUrl(url string) {
	exec.Command("/usr/bin/xdg-open", url).Run()
	// Support for WSL
	exec.Command("/usr/bin/env", "cmd.exe", "/C", "start", url).Run()
}
