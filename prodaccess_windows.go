// +build windows

package main
import (
)

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"github.com/dhtech/prodaccess/pageant"
)

var (
	vaultTokenPath  = flag.String("vault_token", "$USERPROFILE\\.vault-token", "Path to Vault token to update.")

	MB_OK               = 0x00000000
	MB_ICONHAND         = 0x00000010
	MB_ICONEXCLAMATION  = 0x00000030
)

func showWarning(msg string) {
	var mod = syscall.NewLazyDLL("user32.dll")
	var proc = mod.NewProc("MessageBoxW")

	proc.Call(0,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(msg))),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("Prodaccess warning"))),
		uintptr(MB_OK | MB_ICONEXCLAMATION))
}

func showError(msg string) {
	var mod = syscall.NewLazyDLL("user32.dll")
	var proc = mod.NewProc("MessageBoxW")

	proc.Call(0,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(msg))),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("Prodaccess error"))),
		uintptr(MB_OK | MB_ICONHAND))
}

func openUrl(url string) {
	rundll := filepath.Join(os.Getenv("SYSTEMROOT"), "system32", "rundll32.exe")
	exec.Command(rundll, "url.dll,FileProtocolHandler", url).Run()
}

func sshGetPublicKey() (string, error) {
	if !pageant.Available() {
		showWarning("No pageant detected, will not request SSH certificate")
		return "", fmt.Errorf("no pageant")
	}

	keys, err := pageant.New().List()
	if err != nil {
		showWarning("Failed to get keys from Pageant")
		return "", fmt.Errorf("no pageant")
	}

	// Pick the first non-certificate key
	for _, key := range keys {
		if strings.Contains(key.Type(), "-cert-v01@openssh.com") {
			continue
		}
		// Our hacked pageant only supports certificates of ECDSA for now
		if !strings.Contains(key.Type(), "ecdsa-sha2-nistp") {
			continue
		}
		return key.String(), nil
	}

	showWarning("Did not find any signable ECDSA keys in your Pageant, will not request SSH certificate")
	return "", fmt.Errorf("no keys found")
}

func sshLoadCertificate(c string) {
	if !pageant.Available() {
		return
	}

	err := pageant.New().LoadHackCertificate(c)
	if err != nil {
		showError(fmt.Sprintf("Failed to add key to Pageant: %v", err))
		return
	}
}

func saveVaultToken(t string) {
	tp := os.ExpandEnv(*vaultTokenPath)
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

	exec.Command("kubectl", "config", "set-credentials",
		"dhtech", "--embed-certs=true",
		fmt.Sprintf("--client-certificate=%s", cp),
		fmt.Sprintf("--client-key=%s", kp)).Run()
	os.Remove(cp)
	os.Remove(kp)
}

func saveBrowserCertificate(c string, k string) {
	log.Printf("saveBrowserCertificate not implemented")
}

func saveVmwareCertificate(c string, k string) {
	log.Printf("saveVmwareCertificate not implemented")
}
