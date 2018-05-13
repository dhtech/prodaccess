// +build freebsd linux

package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
)

var (
	sshPubKey       = flag.String("sshpubkey", "$HOME/.ssh/id_ecdsa.pub", "SSH public key to request signed.")
	sshCert         = flag.String("sshcert", "$HOME/.ssh/id_ecdsa-cert.pub", "SSH certificate to write.")
	vaultTokenPath  = flag.String("vault_token", "$HOME/.vault-token", "Path to Vault token to update.")
)

func sshLoadCertificate(c string) {
	cp := os.ExpandEnv(*sshCert)
	err := ioutil.WriteFile(cp, []byte(c), 0644)
	if err != nil {
		log.Printf("failed to write SSH certificate: %v", err)
	}

	// OpenSSH requires adding the private key again to load certificates
	pp := strings.TrimSuffix(cp, "-cert.pub")
	exec.Command("/usr/bin/env", "ssh-add", pp).Run()
}

func sshGetPublicKey() (string, error) {
	key, err := ioutil.ReadFile(os.ExpandEnv(*sshPubKey))
	if err != nil {
		log.Printf("could not read SSH public key: %v", err)
		return "", err
	}
	return string(key), nil
}

func saveVaultToken(t string) {
	tp := os.ExpandEnv(*vaultTokenPath)
	err := ioutil.WriteFile(tp, []byte(t), 0400)
	if err != nil {
		log.Printf("failed to write Vault token: %v", err)
	}
}
