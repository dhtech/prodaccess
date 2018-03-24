package main

import (
	"log"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	pb "github.com/dhtech/proto/auth"
)

const (
	address     = "localhost:1214"
)

func main() {
	// Set up a connection to the server.
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := pb.NewAuthenticationServiceClient(conn)
	
	key := "ssh-rsa AAAAB3NzaC1yc2EAAAABJQAAAQEA4FxXEQsHdk6LknY1O6vjMZ880m7VN9QJVOAI+yoF08Ot70e3G0wbddKIuM53ZBNk2484M0A6bTK09e475XQegqkObMouZ/QNVlK/hyEMUg+lutwjCpDbQ3NFzvKwqk0w/LbLUmZp2PpKYpp5kmYzbDkAvGlKuIP2m2BuukRyrG6HTYmdFGc5VJDeeqkhdBcF25n6afPLVfsCK8O+PBcijBAHmuTRHWfrn1zhRglDeb9lRdX+9twLCt0ruveFGzRdR5Bv5y6yIKiAz3VqO9A1PZXX5sVNXDQw4EQmlrZ3o/nYXVuClrz4ULGvTzOujhDZnkIEGTjZV8P02He5gwSSmQ== rsa-key-20151019"
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	
    stream, err := c.RequestUserCredential(ctx, &pb.UserCredentialRequest{
		SshCertificateRequest: &pb.SshCertificateRequest{
			PublicKey: key,
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
			openUrl("http://localhost" + response.RequiredAction.Url)
		} else {
			break
		}
		response, err = stream.Recv()
	}

	log.Printf("Got credentials: %v", response)
}
