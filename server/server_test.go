package mvcamctrl

import (
	"context"
	"errors"
	"fmt"
	"github.com/wuyuanyi135/mvprotos/mvpulse"
	"google.golang.org/grpc"
	"net"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	go StartTestServer(context.Background())
	client, err := GetClient()
	if err != nil {
		panic(err)
	}
	disconnect(client)
	m.Run()
}

func StartTestServer(ctx context.Context) {
	grpcServer := grpc.NewServer()
	mvpulse.RegisterMicroVisionPulseServiceServer(grpcServer, NewPulseSerice())

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

func ConnectDevice(client mvpulse.MicroVisionPulseServiceClient) error {
	serialDevices, err := client.GetDevices(context.Background(), &mvpulse.GetDevicesReq{})
	if err != nil {
		return err
	}
	devList := serialDevices.Devices

	if len(devList) == 0 {
		return errors.New("no device")
	}
	selectedDevice := devList[0]

	_, err = client.Connect(context.Background(), &mvpulse.ConnectReq{
		DeviceIdentifier: &mvpulse.ConnectReq_Path{Path: selectedDevice.Path},
	})

	if err != nil {
		return err
	}
	return nil
}

func TestLaserCtrlServer_GetDriverVersion(t *testing.T) {
	client, err := GetClient()

	if err != nil {
		t.Fatal(err)
	}

	response, err := client.DriverVersion(context.Background(), &mvpulse.DriverVersionReq{})
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
	connect(client)
	defer disconnect(client)

	//<-time.After(time.Second) // TODO: after connection it seems the message cannot be sent immediately?

	deviceResponse, err := client.DeviceVersion(context.Background(), &mvpulse.DeviceVersionReq{})
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Hardware Version = %d, Firmware Version = %d\n", deviceResponse.HardwareVersion, deviceResponse.FirmwareVersion)
}

func TestLaserCtrlServer_SetGetPower(t *testing.T) {
	client, err := GetClient()
	if err != nil {
		t.Fatal(err)
	}

	connect(client)
	defer disconnect(client)

	// default power state check
	power, err := client.GetPower(context.Background(), &mvpulse.GetPowerReq{})
	if err != nil {
		t.Fatal(err)
	}

	p := power.Power.MasterPower
	if p {
		t.Fatalf("Default power state should be off, got %t", p)
	}

	// set power on
	_, err = client.SetPower(context.Background(), &mvpulse.SetPowerReq{
		Power: &mvpulse.PowerConfiguration{MasterPower: true},
	})
	if err != nil {
		t.Fatal(err)
	}

	power, err = client.GetPower(context.Background(), &mvpulse.GetPowerReq{})
	if err != nil {
		t.Fatal(err)
	}
	p = power.Power.MasterPower
	if !p {
		t.Fatalf("Power state should be on, got %t", p)
	}

	fmt.Println("Please check whether the laser fan is running")
	<-time.After(time.Second * 5)

	// set power off
	_, err = client.SetPower(context.Background(), &mvpulse.SetPowerReq{
		Power: &mvpulse.PowerConfiguration{MasterPower: false},
	})
	if err != nil {
		t.Fatal(err)
	}
	power, err = client.GetPower(context.Background(), &mvpulse.GetPowerReq{})
	if err != nil {
		t.Fatal(err)
	}
	p = power.Power.MasterPower
	if p {
		t.Fatalf("Power state should be off, got %t", p)
	}

	fmt.Println("Please check whether the laser fan is off")
	<-time.After(time.Second * 3)
}

func TestLaserCtrlServer_CommitParameter(t *testing.T) {
	client, err := GetClient()
	if err != nil {
		t.Fatal(err)
	}

	connect(client)
	defer disconnect(client)

	// check default parameters
	defaultParameters, err := client.GetPulseParam(context.Background(), &mvpulse.GetPulseParamReq{})
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Default parameters: %v", defaultParameters)

	modifiedParameters := mvpulse.PulseConfiguration{
		PulseDelay:    10,
		DigitalFilter: 2,
		ExposureTick:  720,
	}

	_, err = client.SetPulseParam(
		context.Background(),
		&mvpulse.SetPulseParamReq{
			Pulse: &modifiedParameters,
		},
	)

	if err != nil {
		t.Fatal(err)
	}

	_, err = client.CommitParameter(
		context.Background(),
		&mvpulse.CommitParameterReq{},
	)
	if err != nil {
		t.Fatal(err)
	}

	// value should have been changed.
	resp, err := client.GetPulseParam(context.Background(), &mvpulse.GetPulseParamReq{})
	if err != nil {
		t.Fatal(err)
	}
	params := resp.Pulse
	if params.ExposureTick != modifiedParameters.ExposureTick ||
		params.DigitalFilter != modifiedParameters.DigitalFilter ||
		params.PulseDelay != modifiedParameters.PulseDelay {
		t.Fatalf("Value is not updated after commitment: got= %v, expected=%v", params, modifiedParameters)
	}

	// restore default
	_, err = client.SetPulseParam(
		context.Background(),
		&mvpulse.SetPulseParamReq{
			Pulse:  defaultParameters.Pulse,
			Commit: true,
		},
	)

	if err != nil {
		t.Fatal(err)
	}
}

func TestParameterStreaming(t *testing.T) {
	client, err := GetClient()
	if err != nil {
		t.Fatal(err)
	}

	streamingClient, err := client.ParameterStreaming(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// test connect message
	t.Log("Test connect message")
	{
		go func() {
			<-time.After(1 * time.Second)
			connect(client)
		}()
		stream, err := streamingClient.Recv()
		if err != nil {
			t.Fatal(err)
		}
		if !stream.Opened {
			t.Fatal("Open message not received")
		}
	}

	// test trigger arm
	t.Log("Test trigger message")
	{
		go func() {
			<-time.After(1 * time.Second)
			_, err := client.SetTriggerArm(context.Background(), &mvpulse.SetTriggerArmReq{ArmTrigger: true})
			if err != nil {
				t.Fatal(err)
			}
		}()
		stream, err := streamingClient.Recv()
		if err != nil {
			t.Fatal(err)
		}
		if !stream.TriggerArmed {
			t.Fatal("trigger not armed")
		}
	}
	{
		go func() {
			<-time.After(1 * time.Second)
			_, err := client.SetTriggerArm(context.Background(), &mvpulse.SetTriggerArmReq{ArmTrigger: false})
			if err != nil {
				t.Fatal(err)
			}
		}()
		stream, err := streamingClient.Recv()
		if err != nil {
			t.Fatal(err)
		}
		if stream.TriggerArmed {
			t.Fatal("trigger not disarmed")
		}
	}

	// test set power
	t.Log("Test power message")
	{
		go func() {
			<-time.After(1 * time.Second)
			err := streamingClient.Send(&mvpulse.ParameterStream{Power: &mvpulse.PowerConfiguration{MasterPower: true}})
			if err != nil {
				t.Fatal(err)
			}
		}()
		stream, err := streamingClient.Recv()
		if err != nil {
			t.Fatal(err)
		}
		if !stream.Power.MasterPower {
			t.Fatal("power not set")
		}
	}
	{
		go func() {
			<-time.After(1 * time.Second)
			err := streamingClient.Send(&mvpulse.ParameterStream{Power: &mvpulse.PowerConfiguration{MasterPower: false}})
			if err != nil {
				t.Fatal(err)
			}
		}()
		stream, err := streamingClient.Recv()
		if err != nil {
			t.Fatal(err)
		}
		if stream.Power.MasterPower {
			t.Fatal("power not unset")
		}
	}

	// parameter
	t.Log("Test parameter message")
	{
		go func() {
			<-time.After(1 * time.Second)
			err := streamingClient.Send(&mvpulse.ParameterStream{Pulse: &mvpulse.PulseConfiguration{ExposureTick: 650}})
			if err != nil {
				t.Fatal(err)
			}
		}()
		stream, err := streamingClient.Recv()
		if err != nil {
			t.Fatal(err)
		}
		if stream.Pulse.ExposureTick != 650 {
			t.Fatal("pulse not unset")
		}
	}

	// disconnect
	t.Log("Test disconnect message")
	{
		go func() {
			<-time.After(1 * time.Second)
			disconnect(client)
		}()
		stream, err := streamingClient.Recv()
		if err != nil {
			t.Fatal(err)
		}
		if stream.Opened {
			t.Fatal("disconnect not received")
		}
	}
}

// boilerplate code
func disconnect(client mvpulse.MicroVisionPulseServiceClient) {
	_, err := client.Disconnect(context.Background(), &mvpulse.DisconnectReq{})
	if err != nil {
		panic(err)
	}
}
func connect(client mvpulse.MicroVisionPulseServiceClient) {
	err := ConnectDevice(client)
	if err != nil {
		panic(err)
	}
}
func GetClient() (mvpulse.MicroVisionPulseServiceClient, error) {
	var err error
	if conn == nil {
		conn, err = grpc.Dial("localhost:3050", grpc.WithInsecure())
	}
	if err != nil {
		return nil, err
	}

	return mvpulse.NewMicroVisionPulseServiceClient(conn), nil
}
