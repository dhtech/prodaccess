// +build freebsd

package main

import (
	"os/exec"
)

func openUrl(url string) {
	exec.Command("/usr/local/bin/xdg-open", url).Run()
}