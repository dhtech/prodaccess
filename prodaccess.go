package main

import (
	"log"
	"io"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	pb "github.com/dhtech/protos/auth"
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	
    stream, err := c.RequestCredential(ctx, &pb.NewCredentialRequest{
		SshCertificateRequest: &pb.SshCertificateRequest{
			PublicKey: key,
		},
	})
	
	if err != nil {
		log.Fatalf("could not request credentials: %v", err)
	}
	
	for {
		response, err := stream.Recv()
		if (err == io.EOF) {
			break;
		}
		log.Printf("Greeting: %v", response)
	}
}
