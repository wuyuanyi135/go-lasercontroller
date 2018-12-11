package main

import (
	"fmt"
	"github.com/wuyuanyi135/lasercontroller/server"
)

func main() {
	fmt.Println("Starting server")
	laserctrlgrpc.StartServer()
}
