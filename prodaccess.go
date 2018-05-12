package main

import (
	"crypto/tls"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	pb "github.com/dhtech/proto/auth"
)

var (
	grpcAddress  = flag.String("grpc", "auth.tech.dreamhack.se:443", "Authentication server to use.")
	useTls       = flag.Bool("tls", true, "Whether or not to use TLS for the GRPC connection")
	webUrl       = flag.String("web", "https://auth.tech.dreamhack.se", "Domain to reply to ident requests from")
	ident        = ""
	sshPubKey    = flag.String("sshpubkey", "$HOME/.ssh/id_ecdsa.pub", "SSH public key to request signed.")
	sshCert      = flag.String("sshcert", "$HOME/.ssh/id_ecdsa-cert.pub", "SSH certificate to write.")
)

func presentIdent(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain")
	w.Header().Add("Access-Control-Allow-Origin", *webUrl)
	w.Write([]byte(ident))
}

func main() {
	flag.Parse()

	// Create ident server, used to validate requests to protect from crosslinking.
	ident = uuid.New().String()
	http.HandleFunc("/", presentIdent)
	go http.ListenAndServe(":1215", nil)

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
	
	key, err := ioutil.ReadFile(os.ExpandEnv(*sshPubKey))
	if err != nil {
		log.Fatalf("could not read SSH public key: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	
		stream, err := c.RequestUserCredential(ctx, &pb.UserCredentialRequest{
			ClientValidation: &pb.ClientValidation{
				Ident: ident,
			},
			SshCertificateRequest: &pb.SshCertificateRequest{
				PublicKey: string(key),
			},
	})
	
	if err != nil {
		log.Fatalf("could not request credentials: %v", err)
	}
	
	response, err := stream.Recv()
	for {
		if (err != nil) {
			log.Printf("Error: %v", err)
			break
		}
		log.Printf("Response: %v", response)
		if (response.RequiredAction != nil) {
			openUrl(*webUrl + response.RequiredAction.Url)
		} else {
			break
		}
		response, err = stream.Recv()
	}

	if response.SshCertificate != nil {
		cp := os.ExpandEnv(*sshCert)
		err := ioutil.WriteFile(cp, []byte(response.SshCertificate.Certificate), 0644)
		if err != nil {
			log.Printf("failed to write SSH certificate: %v", err)
		}
		sshAgentAdd(cp)
	}
}
