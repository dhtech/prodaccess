// +build linux

package main

import (
	"os/exec"
	"strings"
)

func openUrl(url string) {
	exec.Command("/usr/bin/xdg-open", url).Run()
}

func sshAgentAdd(cp string) {
	// OpenSSH requires adding the private key again to load certificates
	cp = strings.TrimSuffix(cp, "-cert.pub")
	exec.Command("/usr/bin/env", "ssh-add", cp).Run()
}
