package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	url "github.com/dhtech/go-openurl"
	"github.com/google/uuid"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	pb "github.com/dhtech/proto/auth"
)

var (
	grpcAddress    = flag.String("grpc", "auth.tech.dreamhack.se:443", "Authentication server to use.")
	useTls         = flag.Bool("tls", true, "Whether or not to use TLS for the GRPC connection")
	webUrl         = flag.String("web", "https://auth.tech.dreamhack.se", "Domain to reply to ident requests from")
	requestVmware  = flag.Bool("vmware", false, "Whether or not to request a VMware certificate")
	// TODO(bluecmd): This should be automatic
	requestBrowser = flag.Bool("browser", false, "Whether or not to request a browser certificate")
	rsaKeySize     = flag.Int("rsa_key_size", 4096, "When generating RSA keys, use this key size")
	ident          = ""
)

func presentIdent(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain")
	w.Header().Add("Access-Control-Allow-Origin", *webUrl)
	w.Write([]byte(ident))
}

func quit(w http.ResponseWriter, r *http.Request) {
	// This is used to kill any other prodaccess that is lingering, enforcing
	// that only one is running.
	log.Printf("Got termination request by /quit")
	os.Exit(0)
}

func mustServeHttp() {
	err := http.ListenAndServe(":1215", nil)
	if err != nil {
		log.Fatalf("could not serve backend http: %v", err)
	}
}

func generateEcdsaCsr() (string, string, error) {
	keyb, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	if err != nil {
		return "", "", err
	}
	asnKey, err := x509.MarshalECPrivateKey(keyb)
	if err != nil {
		return "", "", err
	}
	keyPemBlob := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: asnKey})

	subj := pkix.Name{
		CommonName: "replaced-by-the-server",
	}
	tmpl := x509.CertificateRequest{
		Subject: subj,
		SignatureAlgorithm: x509.ECDSAWithSHA256,
	}
	csrb, _ := x509.CreateCertificateRequest(rand.Reader, &tmpl, keyb)
	pemBlob := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrb})

	return string(keyPemBlob), string(pemBlob), nil
}

func generateRsaCsr() (string, string, error) {
	keyb, err := rsa.GenerateKey(rand.Reader, *rsaKeySize)
	if err != nil {
		return "", "", err
	}
	asnKey := x509.MarshalPKCS1PrivateKey(keyb)
	keyPemBlob := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: asnKey})

	subj := pkix.Name{
		CommonName: "replaced-by-the-server",
	}
	tmpl := x509.CertificateRequest{
		Subject: subj,
		SignatureAlgorithm: x509.SHA256WithRSA,
	}
	csrb, _ := x509.CreateCertificateRequest(rand.Reader, &tmpl, keyb)
	pemBlob := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrb})

	return string(keyPemBlob), string(pemBlob), nil
}

func main() {
	flag.Parse()

	// Attempt to kill any already running prodaccess
	http.Get("http://localhost:1215/quit")

	// Create ident server, used to validate requests to protect from crosslinking.
	ident = uuid.New().String()
	http.HandleFunc("/", presentIdent)
	http.HandleFunc("/quit", quit)
	go mustServeHttp()

	d := grpc.WithInsecure()
	if *useTls {
		d = grpc.WithTransportCredentials(
			credentials.NewTLS(&tls.Config{}),
		)
	}

	// Set up a connection to the server.
	conn, err := grpc.Dial(*grpcAddress, d)
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := pb.NewAuthenticationServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	ucr := &pb.UserCredentialRequest{
			ClientValidation: &pb.ClientValidation{
				Ident: ident,
			},
			VaultTokenRequest: &pb.VaultTokenRequest{},
	}

	vmwarePk := ""
	if *requestVmware {
		csr := ""
		log.Printf("Generating VMware CSR ...")
		vmwarePk, csr, err = generateEcdsaCsr()
		if err != nil {
			log.Fatalf("failed to generate VMware CSR: %v", err)
		}
		ucr.VmwareCertificateRequest = &pb.VmwareCertificateRequest{
			Csr: csr,
		}
	}

	browserPk := ""
	if *requestBrowser {
		csr := ""
		log.Printf("Generating Browser CSR ...")
		browserPk, csr, err = generateEcdsaCsr()
		if err != nil {
			log.Fatalf("failed to generate Browser CSR: %v", err)
		}
		ucr.BrowserCertificateRequest = &pb.BrowserCertificateRequest{
			Csr: csr,
		}
	}

	if hasKubectl() {
		ucr.KubernetesCertificateRequest = &pb.KubernetesCertificateRequest{}
	}

	sshPkey, err := sshGetPublicKey()
	if err == nil {
		ucr.SshCertificateRequest = &pb.SshCertificateRequest{
			PublicKey: sshPkey,
		}
	}

	log.Printf("Sending credential request")
	stream, err := c.RequestUserCredential(ctx, ucr)
	if err != nil {
		log.Fatalf("could not request credentials: %v", err)
	}

	response, err := stream.Recv()
	for {
		if (err != nil) {
			log.Printf("Error: %v", err)
			break
		}
		if (response.RequiredAction != nil) {
			log.Printf("Required action: %v", response.RequiredAction)
			url.Open(*webUrl + response.RequiredAction.Url)
		} else {
			break
		}
		response, err = stream.Recv()
	}

	if response.SshCertificate != nil {
		sshLoadCertificate(response.SshCertificate.Certificate)
	}

	if response.VaultToken != nil {
		saveVaultToken(response.VaultToken.Token)
	}

	if response.KubernetesCertificate != nil {
		saveKubernetesCertificate(response.KubernetesCertificate.Certificate, response.KubernetesCertificate.PrivateKey)
	}

	if response.VmwareCertificate != nil {
		full := append([]string{response.VmwareCertificate.Certificate}, response.VmwareCertificate.CaChain...)
		saveVmwareCertificate(strings.Join(full, "\n"), vmwarePk)
	}

	if response.BrowserCertificate != nil {
		full := append([]string{response.BrowserCertificate.Certificate}, response.BrowserCertificate.CaChain...)
		saveBrowserCertificate(strings.Join(full, "\n"), browserPk)
	}
}
