package laserctrlgrpc

import (
	"context"
	"encoding/binary"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/op/go-logging"
	"github.com/wuyuanyi135/lasercontroller/protos"
	"github.com/wuyuanyi135/lasercontroller/serial"
	"github.com/wuyuanyi135/lasercontroller/serial/command"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"time"
)

const DriverVersion = "1.0"

var log = logging.MustGetLogger("Server")

type LaserCtrlServer struct {
	serialInstance serial.Serial
}

func New() *LaserCtrlServer {
	return &LaserCtrlServer{serialInstance: serial.NewSerial()}
}

func (LaserCtrlServer) GetSerialDevices(context.Context, *empty.Empty) (*laserctrlgrpc.SerialListResponse, error) {
	resp := laserctrlgrpc.SerialListResponse{}

	devList, err := serial.ListSerialPorts()
	if err != nil {
		return &resp, err
	}

	for name, destination := range devList {
		resp.DeviceList = append(resp.DeviceList, &laserctrlgrpc.SerialDeviceMapping{Name: name, Destination: destination})
	}

	return &resp, nil
}

func (LaserCtrlServer) GetDriverVersion(context.Context, *empty.Empty) (*laserctrlgrpc.DriverVersionResponse, error) {
	resp := laserctrlgrpc.DriverVersionResponse{
		Version: DriverVersion,
	}

	return &resp, nil
}

func (s *LaserCtrlServer) Connect(ctx context.Context, request *laserctrlgrpc.ConnectRequest) (*laserctrlgrpc.EmptyResponse, error) {
	var err error
	switch request.DeviceIdentifier.(type) {
	case *laserctrlgrpc.ConnectRequest_Path:
		err = s.serialInstance.ConnectByPath(request.GetPath())
	case *laserctrlgrpc.ConnectRequest_Name:
		err = s.serialInstance.ConnectByName(request.GetName())
	}

	var resp laserctrlgrpc.EmptyResponse

	return &resp, err
}

func (s *LaserCtrlServer) Disconnect(context.Context, *empty.Empty) (*laserctrlgrpc.EmptyResponse, error) {
	err := s.serialInstance.Disconnect()

	var resp laserctrlgrpc.EmptyResponse
	return &resp, err
}

func (s *LaserCtrlServer) deviceRequest(ctx context.Context, command serial.SerialCommand) ([]byte, error) {
	if ctx != nil {
		command.Ctx = ctx
	}

	if command.ResponseChannel == nil {
		command.ResponseChannel = make(chan []byte)
	}

	err := s.serialInstance.WriteCommandAndRegisterResponse(command)
	if err != nil {
		return nil, err
	}

	select {
	case response := <-command.ResponseChannel:
		return response, nil
	case <-ctx.Done():
		return nil, status.Errorf(codes.DeadlineExceeded, "%#v command time out", command)
	}
}

func (s *LaserCtrlServer) GetDeviceVersion(ctx context.Context, req *empty.Empty) (*laserctrlgrpc.DeviceVersionResponse, error) {
	var resp = laserctrlgrpc.DeviceVersionResponse{}
	ctx, _ = context.WithTimeout(ctx, time.Second)
	version, err := s.deviceRequest(
		ctx,
		serial.SerialCommand{
			Command: command.CommandVersion,
			Arg:     nil,
		},
	)
	if err != nil {
		log.Errorf("Get device version error: %s", err.Error())
		return nil, err
	}

	resp.HardwareVersion = uint32(version[0])
	resp.FirmwareVersion = uint32(version[1])

	return &resp, nil
}

func (s *LaserCtrlServer) SetPower(ctx context.Context, req *laserctrlgrpc.SetPowerRequest) (*laserctrlgrpc.EmptyResponse, error) {
	resp := laserctrlgrpc.EmptyResponse{}

	ctx, _ = context.WithTimeout(ctx, time.Second)
	var power byte = 0
	if req.Power.MasterPower {
		power = 1
	}
	_, err := s.deviceRequest(
		ctx,
		serial.SerialCommand{
			Command: command.CommandSetPower,
			Arg:     []byte{power},
		})
	if err != nil {
		log.Errorf("Set power error: %s", err.Error())
		return nil, err
	}

	return &resp, nil
}

func (s *LaserCtrlServer) GetPower(ctx context.Context, req *empty.Empty) (*laserctrlgrpc.PowerConfiguration, error) {
	resp := laserctrlgrpc.PowerConfiguration{}

	ctx, _ = context.WithTimeout(ctx, time.Second)
	power, err := s.deviceRequest(ctx, serial.SerialCommand{
		Command: command.CommandGetPower,
	})
	if err != nil {
		log.Errorf("Get power error: %s", err.Error())
		return nil, err
	}

	resp.MasterPower = power[0] == 1
	return &resp, nil
}

func (s *LaserCtrlServer) SetLaserParam(ctx context.Context, req *laserctrlgrpc.SetLaserRequest) (*laserctrlgrpc.EmptyResponse, error) {
	resp := laserctrlgrpc.EmptyResponse{}
	ctx, _ = context.WithTimeout(ctx, time.Second)

	config := req.Laser

	b := make([]byte, command.CommandSetExposure.RequestLength)
	binary.LittleEndian.PutUint16(b, uint16(config.ExposureTick))
	_, err := s.deviceRequest(
		ctx,
		serial.SerialCommand{
			Command: command.CommandSetExposure,
			Arg:     b,
		},
	)
	if err != nil {
		log.Errorf("Failed to set exposure: %s", err)
		return nil, err
	}

	b = make([]byte, command.CommandSetFilter.RequestLength)
	binary.LittleEndian.PutUint16(b, uint16(config.DigitalFilter))
	_, err = s.deviceRequest(
		ctx,
		serial.SerialCommand{
			Command: command.CommandSetFilter,
			Arg:     b,
		},
	)
	if err != nil {
		log.Errorf("Failed to set filter: %s", err)
		return nil, err
	}

	b = make([]byte, command.CommandSetDelay.RequestLength)
	binary.LittleEndian.PutUint16(b, uint16(config.PulseDelay))
	_, err = s.deviceRequest(
		ctx,
		serial.SerialCommand{
			Command: command.CommandSetDelay,
			Arg:     b,
		},
	)
	if err != nil {
		log.Errorf("Failed to set exposure: %s", err)
		return nil, err
	}

	return &resp, nil
}

func (s *LaserCtrlServer) GetLaserParam(ctx context.Context, req *empty.Empty) (*laserctrlgrpc.LaserConfiguration, error) {
	resp := laserctrlgrpc.LaserConfiguration{}
	ctx, _ = context.WithTimeout(ctx, time.Second)
	// exposure
	b := make([]byte, command.CommandGetExposure.ResponseLength)

	param, err := s.deviceRequest(
		ctx,
		serial.SerialCommand{
			Command: command.CommandGetExposure,
			Arg:     b,
		},
	)
	if err != nil {
		log.Errorf("Failed to get exposure: %s", err)
		return nil, err
	}

	resp.ExposureTick = uint32(binary.LittleEndian.Uint16(param))

	// filter
	b = make([]byte, command.CommandGetFilter.ResponseLength)

	param, err = s.deviceRequest(
		ctx,
		serial.SerialCommand{
			Command: command.CommandGetFilter,
			Arg:     b,
		},
	)
	if err != nil {
		log.Errorf("Failed to get filter: %s", err)
		return nil, err
	}

	resp.DigitalFilter = uint32(binary.LittleEndian.Uint16(param))

	// delay
	b = make([]byte, command.CommandGetDelay.ResponseLength)

	param, err = s.deviceRequest(
		ctx,
		serial.SerialCommand{
			Command: command.CommandGetDelay,
			Arg:     b,
		},
	)
	if err != nil {
		log.Errorf("Failed to get delay: %s", err)
		return nil, err
	}

	resp.PulseDelay = uint32(binary.LittleEndian.Uint16(param))

	return &resp, nil
}

func (s *LaserCtrlServer) CommitParameter(ctx context.Context, req *empty.Empty) (*laserctrlgrpc.EmptyResponse, error) {
	resp := laserctrlgrpc.EmptyResponse{}
	ctx, _ = context.WithTimeout(ctx, time.Second)

	_, err := s.deviceRequest(ctx, serial.SerialCommand{
		Command: command.CommandCommitParameters,
	})

	if err != nil {
		log.Errorf("Failed to commit parameter: %s", err.Error())
		return nil, err
	}
	return &resp, err
}

func (LaserCtrlServer) ControlLaser(context.Context, *laserctrlgrpc.ControlLaserRequest) (*laserctrlgrpc.EmptyResponse, error) {
	panic("implement me")
}

func (LaserCtrlServer) GetStatus(context.Context, *empty.Empty) (*laserctrlgrpc.Status, error) {
	panic("implement me")
}
func (s LaserCtrlServer) ResetController(context.Context, *empty.Empty) (*laserctrlgrpc.EmptyResponse, error) {
	panic("implement me")
}
