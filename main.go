package main

import (
	"fmt"
	"github.com/wuyuanyi135/lasercontroller/server"
)

//go:generate sh -c "protoc -I protos/MicroVision-proto/laser-controller protos/MicroVision-proto/laser-controller/*.proto --go_out=plugins=grpc:server/protogen"
func main() {
	fmt.Println("Starting server")
	laserctrlgrpc.StartServer()
}
