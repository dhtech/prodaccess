// +build freebsd linux darwin

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
)

var (
	sshPubKey       = flag.String("sshpubkey", "$HOME/.ssh/id_ecdsa.pub", "SSH public key to request signed")
	sshCert         = flag.String("sshcert", "$HOME/.ssh/id_ecdsa-cert.pub", "SSH certificate to write")
	vaultTokenPath  = flag.String("vault_token", "$HOME/.vault-token", "Path to Vault token to update")
	vmwareCertPath  = flag.String("vmware_cert_path", "$HOME/.vmware-user.pfx", "Path to store VMware user certificate")
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
	os.Remove(tp)
	err := ioutil.WriteFile(tp, []byte(t), 0400)
	if err != nil {
		log.Printf("failed to write Vault token: %v", err)
	}
}

func hasKubectl() bool {
	_, err := exec.LookPath("kubectl")
	if err != nil {
		return false
	}
	return true
}

func saveKubernetesCertificate(c string, k string) {
	cf, _ := ioutil.TempFile("", "prodaccess-k8s")
	kf, _ := ioutil.TempFile("", "prodaccess-k8s")
	cf.Write([]byte(c))
	kf.Write([]byte(k))
	cp := cf.Name()
	kp := kf.Name()
	cf.Close()
	kf.Close()

	exec.Command("/usr/bin/env", "kubectl", "config", "set-credentials",
		"dhtech", "--embed-certs=true",
		fmt.Sprintf("--client-certificate=%s", cp),
		fmt.Sprintf("--client-key=%s", kp)).Run()
	os.Remove(cp)
	os.Remove(kp)
}

func saveVmwareCertificate(c string, k string) {
	cf, _ := ioutil.TempFile("", "prodaccess-vmware")
	kf, _ := ioutil.TempFile("", "prodaccess-vmware")
	cf.Write([]byte(c))
	kf.Write([]byte(k))
	cp := cf.Name()
	kp := kf.Name()
	cf.Close()
	kf.Close()

	exec.Command("/usr/bin/env", "openssl", "pkcs12", "-export", "-password", "pass:",
		"-in", cp, "-inkey", kp, "-out", os.ExpandEnv(*vmwareCertPath)).Run()
	os.Remove(cp)
	os.Remove(kp)
}
