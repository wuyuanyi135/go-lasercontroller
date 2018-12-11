package main

import (
	"fmt"
	"github.com/wuyuanyi135/lasercontroller/server"
)

//go:generate protoc -I protos protos/*.proto --go_out=plugins=grpc:protos
func main() {
	fmt.Println("Starting server")
	laserctrlgrpc.StartServer()
}
