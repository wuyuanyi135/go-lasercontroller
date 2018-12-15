package laserctrlgrpc

import (
	"github.com/wuyuanyi135/lasercontroller/protos"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"net"
)

func StartServer() {
	grpcServer := grpc.NewServer()
	laserctrlgrpc.RegisterLaserControlServiceServer(grpcServer, &LaserCtrlServer{})
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
