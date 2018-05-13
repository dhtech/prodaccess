package main

import (
	"crypto/tls"
	"flag"
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
	}

	sshPkey, err := sshGetPublicKey()
	if err == nil {
		ucr.SshCertificateRequest = &pb.SshCertificateRequest{
			PublicKey: sshPkey,
		}
	}

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
		log.Printf("Response: %v", response)
		if (response.RequiredAction != nil) {
			openUrl(*webUrl + response.RequiredAction.Url)
		} else {
			break
		}
		response, err = stream.Recv()
	}

	if response.SshCertificate != nil {
		sshLoadCertificate(response.SshCertificate.Certificate)
	}
}
