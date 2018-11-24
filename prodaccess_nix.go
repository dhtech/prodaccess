// +build freebsd linux darwin

package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"

	"golang.org/x/sys/unix"
)

var (
	sshPubKey       = flag.String("sshpubkey", "$HOME/.ssh/id_ecdsa.pub", "SSH public key to request signed")
	sshCert         = flag.String("sshcert", "$HOME/.ssh/id_ecdsa-cert.pub", "SSH certificate to write")
	vaultTokenPath  = flag.String("vault_token", "$HOME/.vault-token", "Path to Vault token to update")
	vmwareCertPath  = flag.String("vmware_cert_path", "$HOME/vmware-user.pfx", "Path to store VMware user certificate")
	browserCertPath = flag.String("browser_cert_path", "$HOME/browser-user.pfx", "Path to store Browswer user certificate")
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
	defer os.Remove(cp)
	defer os.Remove(kp)
	cf.Close()
	cf.Close()
	kf.Close()

	fp := os.ExpandEnv(*vmwareCertPath)
	os.Remove(fp)
	os.OpenFile(fp, os.O_CREATE, 0600)

	_, _ = executeWithStdout("openssl", "pkcs12", "-export", "-password", "pass:", "-in", cp, "-inkey", kp, "-out", fp)

	if isWSL() {
		if err := importCertFromWSL(fp); err != nil {
			log.Printf("Failed to import certificate: %v", err)
		}
	}
}

func saveBrowserCertificate(c string, k string) {
	cf, _ := ioutil.TempFile("", "prodaccess-browser")
	kf, _ := ioutil.TempFile("", "prodaccess-browser")
	cf.Write([]byte(c))
	kf.Write([]byte(k))
	cp := cf.Name()
	kp := kf.Name()
	defer os.Remove(cp)
	defer os.Remove(kp)
	cf.Close()
	kf.Close()

	fp := os.ExpandEnv(*browserCertPath)
	os.Remove(fp)
	os.OpenFile(fp, os.O_CREATE, 0600)
	_, _ = executeWithStdout("openssl", "pkcs12", "-export", "-password", "pass:", "-in", cp, "-inkey", kp, "-out", fp)

	if isWSL() {
		if err := importCertFromWSL(fp); err != nil {
			log.Printf("Failed to import certificate: %v", err)
		}
	}
}

func isWSL() bool {
	u := unix.Utsname{}
	_ = unix.Uname(&u)
	return strings.Contains(string(u.Release[:]), "Microsoft")
}

func executeWithStdout(cmd ...string) (string, error) {
	return executeWithStdoutWithStdin("", cmd...)
}

func executeWithStdoutWithStdin(stdin string, cmd ...string) (string, error) {
	c := exec.Command("/usr/bin/env", cmd...)
	var stdout, stderr bytes.Buffer
	si := bytes.NewBufferString(stdin)
	c.Stdout = &stdout
	c.Stderr = &stderr
	c.Stdin = si

	err := c.Run()
	if err != nil {
		log.Printf("Failed to execute %v: %v", cmd, err)
		log.Printf("Standard output: %s", stdout.String())
		log.Printf("Error output: %s", stderr.String())
		return "", err
	}

	return stdout.String(), nil
}

func executeWithStdin(stdin string, cmd ...string) error {
	_, err := executeWithStdoutWithStdin(stdin, cmd...)
	return err
}

// If running under WSL invoke PowerShell to import certificate
func importCertFromWSL(pfx string) error {
	winpath, err := executeWithStdout("powershell.exe", "echo $env:TEMP")
	if err != nil {
		return err
	}

	winpath = strings.Trim(winpath, "\r\n")
	winparts := strings.Split(winpath, "\\")
	drive := strings.ToLower(winparts[0][:1])
	winparts = winparts[1:]
	temppath := path.Join("/mnt", drive, path.Join(winparts...), "prodaccess.pfx")

	pfxdata, err := ioutil.ReadFile(pfx)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(temppath, pfxdata, 0644)
	if err != nil {
		return err
	}
	defer os.Remove(temppath)

	ps := fmt.Sprintf(psImport, winpath+"\\prodaccess.pfx")
	fmt.Printf("%s\n", ps)
	o, err := executeWithStdoutWithStdin(ps, "powershell.exe", "-Command", "-")
	if err != nil {
		return err
	}

	log.Printf("Imported certificate: %s", o)

	return executeWithStdin(psPurge, "powershell.exe", "-Command", "-")
}
