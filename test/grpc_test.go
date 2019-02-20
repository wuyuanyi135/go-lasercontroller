package test

import (
	"context"
	"fmt"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/wuyuanyi135/mvcamctrl/protos"
	"google.golang.org/grpc"
	"testing"
)

func TestGetSerialList(t *testing.T) {
	conn, err := grpc.Dial("localhost:3050", grpc.WithInsecure())
	if err != nil {
		panic("Failed to connect to server")
	}

	client := laserctrlgrpc.NewLaserControlServiceClient(conn)
	response, err := client.GetSerialDevices(context.Background(), &empty.Empty{})
	fmt.Printf("There are %d serial devices\n", len(response.DeviceList))
	for i, element := range response.DeviceList {
		fmt.Printf("#%d: %s=>%s\n", i, element.Name, element.Destination)
	}
}
