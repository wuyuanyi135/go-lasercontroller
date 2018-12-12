package laserctrlgrpc

import (
	"context"
	"fmt"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/wuyuanyi135/lasercontroller/protos"
	"google.golang.org/grpc"
	"net"
	"testing"
	"time"
)

func StartTestServer(ctx context.Context) {
	grpcServer := grpc.NewServer()
	laserctrlgrpc.RegisterLaserControlServiceServer(grpcServer, New())

	lis, err := net.Listen("tcp", ":3050")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	err = grpcServer.Serve(lis)
	if err != nil {
		log.Fatalf("failed to serve: %v", err)
	}

	done := ctx.Done()
	if done != nil {
		<-done
		grpcServer.GracefulStop()
	}
}

var conn *grpc.ClientConn

func GetClient() (laserctrlgrpc.LaserControlServiceClient, error) {
	var err error
	if conn == nil {
		conn, err = grpc.Dial("localhost:3050", grpc.WithInsecure())
	}
	if err != nil {
		return nil, err
	}

	return laserctrlgrpc.NewLaserControlServiceClient(conn), nil
}
func ConnectDevice(client laserctrlgrpc.LaserControlServiceClient) error {
	serialDevices, err := client.GetSerialDevices(context.Background(), &empty.Empty{})
	if err != nil {
		return err
	}
	devList := serialDevices.DeviceList
	var selectedDevice *laserctrlgrpc.SerialDeviceMapping
	if len(devList) >= 1 {
		selectedDevice = devList[0]
	}

	_, err = client.Connect(context.Background(), &laserctrlgrpc.ConnectRequest{
		DeviceIdentifier: &laserctrlgrpc.ConnectRequest_Path{Path: selectedDevice.Destination},
	})

	if err != nil {
		return err
	}
	return nil
}
func TestMain(m *testing.M) {
	go StartTestServer(context.Background())

	m.Run()
}

func TestLaserCtrlServer_GetDriverVersion(t *testing.T) {
	client, err := GetClient()

	if err != nil {
		t.Fatal(err)
	}

	response, err := client.GetDriverVersion(context.Background(), &empty.Empty{})
	if err != nil {
		t.Fatal(err)
	}

	t.Log(response.Version)

	if response.Version != DriverVersion {
		t.Fatal("Version not match")
	}

}

func TestLaserCtrlServer_GetDeviceVersion(t *testing.T) {
	client, err := GetClient()

	if err != nil {
		t.Fatal(err)
	}
	err = ConnectDevice(client)
	if err != nil {
		t.Fatal(err)
	}

	//<-time.After(time.Second) // TODO: after connection it seems the message cannot be sent immediately?

	deviceResponse, err := client.GetDeviceVersion(context.Background(), &empty.Empty{})
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Hardware Version = %d, Firmware Version = %d\n", deviceResponse.HardwareVersion, deviceResponse.FirmwareVersion)

	_, err = client.Disconnect(context.Background(), &empty.Empty{})

	if err != nil {
		t.Fatal(err)
	}
}

func TestLaserCtrlServer_SetGetPower(t *testing.T) {
	client, err := GetClient()
	if err != nil {
		t.Fatal(err)
	}

	err = ConnectDevice(client)
	if err != nil {
		t.Fatal(err)
	}

	// default power state check
	power, err := client.GetPower(context.Background(), &empty.Empty{})
	if err != nil {
		t.Fatal(err)
	}

	if power.MasterPower {
		t.Fatalf("Default power state should be off, got %t", power.MasterPower)
	}

	// set power on
	_, err = client.SetPower(context.Background(), &laserctrlgrpc.SetPowerRequest{
		Power: &laserctrlgrpc.PowerConfiguration{MasterPower: true},
	})
	if err != nil {
		t.Fatal(err)
	}

	power, err = client.GetPower(context.Background(), &empty.Empty{})
	if err != nil {
		t.Fatal(err)
	}
	if !power.MasterPower {
		t.Fatalf("Power state should be on, got %t", power.MasterPower)
	}

	fmt.Println("Please check whether the laser fan is running")
	<-time.After(time.Second * 5)

	// set power off
	_, err = client.SetPower(context.Background(), &laserctrlgrpc.SetPowerRequest{
		Power: &laserctrlgrpc.PowerConfiguration{MasterPower: false},
	})
	if err != nil {
		t.Fatal(err)
	}
	power, err = client.GetPower(context.Background(), &empty.Empty{})
	if err != nil {
		t.Fatal(err)
	}
	if power.MasterPower {
		t.Fatalf("Power state should be off, got %t", power.MasterPower)
	}

	fmt.Println("Please check whether the laser fan is off")
	<-time.After(time.Second * 3)

	_, err = client.Disconnect(context.Background(), &empty.Empty{})
	if err != nil {
		t.Fatal(err)
	}
}

func TestLaserCtrlServer_CommitParameter(t *testing.T) {
	client, err := GetClient()
	if err != nil {
		t.Fatal(err)
	}

	err = ConnectDevice(client)
	if err != nil {
		t.Fatal(err)
	}

	// check default parameters
	defaultParameters, err := client.GetLaserParam(context.Background(), &empty.Empty{})
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Default parameters: %v", defaultParameters)

	modifiedParameters := laserctrlgrpc.LaserConfiguration{
		PulseDelay:    10,
		DigitalFilter: 2,
		ExposureTick:  720,
	}

	_, err = client.SetLaserParam(
		context.Background(),
		&laserctrlgrpc.SetLaserRequest{
			Laser: &modifiedParameters,
		},
	)

	if err != nil {
		t.Fatal(err)
	}

	_, err = client.CommitParameter(
		context.Background(),
		&empty.Empty{},
	)
	if err != nil {
		t.Fatal(err)
	}

	// value should have been changed.
	params, err := client.GetLaserParam(context.Background(), &empty.Empty{})
	if err != nil {
		t.Fatal(err)
	}
	if params.ExposureTick != modifiedParameters.ExposureTick ||
		params.DigitalFilter != modifiedParameters.DigitalFilter ||
		params.PulseDelay != modifiedParameters.PulseDelay {
		t.Fatalf("Value is not updated after commitment: got= %v, expected=%v", params, modifiedParameters)
	}

	// restore default
	_, err = client.SetLaserParam(
		context.Background(),
		&laserctrlgrpc.SetLaserRequest{
			Laser: defaultParameters,
		},
	)

	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Disconnect(context.Background(), &empty.Empty{})
	if err != nil {
		t.Fatal(err)
	}
}
