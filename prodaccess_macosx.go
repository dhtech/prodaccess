// +build darwin

package main

import (
        "os"
        "os/exec"
	"log"
)

func openUrl(url string) {
	var browser string = ""

	if _, err := os.Stat("/Applications/Google Chrome.app"); err == nil {
		// path/to/whatever exists
		browser = "Google Chrome"
	} else if _, err := os.Stat("/Applications/Firefox.app"); err == nil {
		browser = "Firefox"
	} else {
		log.Printf("Error: Chrome or Firefox not found, exiting")		
		os.Exit(1)
	}

	log.Printf("Found browser: %s", browser)
	exec.Command("/usr/bin/open","-a", browser, url).Run()
}
