package mvcamctrl

import (
	"github.com/wuyuanyi135/mvprotos/mvcamctrl"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"net"
)

func StartServer() {
	grpcServer := grpc.NewServer()
	mvcamctrl.RegisterMicroVisionCameraControlServiceServer(grpcServer, New())
	reflection.Register(grpcServer)
	lis, err := net.Listen("tcp", ":3050")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	err = grpcServer.Serve(lis)
	if err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
