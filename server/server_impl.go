package mvcamctrl

import (
	"context"
	"encoding/binary"
	"errors"
	"github.com/olebedev/emitter"
	"github.com/op/go-logging"
	"github.com/wuyuanyi135/mvcamctrl/serial"
	"github.com/wuyuanyi135/mvcamctrl/serial/command"
	"github.com/wuyuanyi135/mvprotos/mvpulse"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"io"
	"time"
)

const DriverVersion = "1.0"

var log = logging.MustGetLogger("Pulse")

type State struct {
	Power         *mvpulse.PowerConfiguration
	Config        *mvpulse.PulseConfiguration
	Opened        bool
	TriggerArmed  bool
	NotifyChanged *emitter.Emitter
	OpenedDevice  *mvpulse.SerialDevice
}

func NewState() *State {

	return &State{
		Opened:        false,
		Config:        nil,
		Power:         nil,
		NotifyChanged: &emitter.Emitter{},
		TriggerArmed:  false,

		OpenedDevice: nil,
	}
}

type PulseSerice struct {
	serialInstance serial.Serial
	State          *State
}

func NewPulseSerice() *PulseSerice {
	return &PulseSerice{
		serialInstance: serial.NewSerial(),
		State:          NewState(),
	}
}

func (s *PulseSerice) GetDevices(context.Context, *mvpulse.GetDevicesReq) (resp *mvpulse.GetDevicesRes, err error) {
	resp = &mvpulse.GetDevicesRes{}

	devList, err := serial.ListSerialPorts()
	if err != nil {
		return
	}

	for name, destination := range devList {
		resp.Devices = append(resp.Devices, &mvpulse.SerialDevice{
			Name: name,
			Path: destination,
		})
	}
	return
}

func (s *PulseSerice) DriverVersion(context.Context, *mvpulse.DriverVersionReq) (resp *mvpulse.DriverVersionRes, err error) {
	resp = &mvpulse.DriverVersionRes{
		Version: DriverVersion,
	}
	return
}

func (s *PulseSerice) Connect(ctx context.Context, req *mvpulse.ConnectReq) (resp *mvpulse.ConnectRes, err error) {
	resp = &mvpulse.ConnectRes{}
	if s.State.Opened && (s.State.OpenedDevice.Name == req.GetName() || s.State.OpenedDevice.Path == req.GetPath()) {
		log.Warning("Repeat open detected. Ignore.")
		return
	}
	var path, name string
	switch req.DeviceIdentifier.(type) {
	case *mvpulse.ConnectReq_Path:
		path = req.GetPath()
		err = s.serialInstance.ConnectByPath(req.GetPath())
		if err != nil {
			return
		}

		mapping, err := serial.ListSerialPorts()
		if err != nil {
			return nil, err
		}
		for k, v := range mapping {
			if v == path {
				name = k
				break
			}
		}
	case *mvpulse.ConnectReq_Name:
		name = req.GetName()
		err, path = s.serialInstance.ConnectByName(req.GetName())
		if err != nil {
			return
		}
	}

	s.State.SetOpened(mvpulse.SerialDevice{
		Path: path,
		Name: name,
	})
	return
}

func (s *PulseSerice) Disconnect(context.Context, *mvpulse.DisconnectReq) (resp *mvpulse.DisconnectRes, err error) {
	resp = &mvpulse.DisconnectRes{}

	err = s.serialInstance.Disconnect()
	if err != nil {
		return
	}

	s.State.SetClosed()
	return
}

func (s *PulseSerice) DeviceVersion(ctx context.Context, req *mvpulse.DeviceVersionReq) (resp *mvpulse.DeviceVersionRes, err error) {
	resp = &mvpulse.DeviceVersionRes{}
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
		return
	}

	resp.HardwareVersion = uint32(version[0])
	resp.FirmwareVersion = uint32(version[1])

	return
}

func (s *PulseSerice) SetPower(ctx context.Context, req *mvpulse.SetPowerReq) (resp *mvpulse.SetPowerRes, err error) {
	err = s.openGuard()
	if err != nil {
		return
	}

	resp = &mvpulse.SetPowerRes{}

	ctx, _ = context.WithTimeout(ctx, time.Second)
	var power byte = 0
	if req.Power.MasterPower {
		power = 1
	}
	_, err = s.deviceRequest(
		ctx,
		serial.SerialCommand{
			Command: command.CommandSetPower,
			Arg:     []byte{power},
		})
	if err != nil {
		log.Errorf("Set power error: %s", err.Error())
		return
	}

	s.State.Power = req.Power
	s.State.NotifyChanged.Emit("status")
	return
}

func (s *PulseSerice) GetPower(ctx context.Context, req *mvpulse.GetPowerReq) (resp *mvpulse.GetPowerRes, err error) {
	err = s.openGuard()
	if err != nil {
		return
	}

	resp = &mvpulse.GetPowerRes{
		Power: &mvpulse.PowerConfiguration{},
	}

	ctx, _ = context.WithTimeout(ctx, time.Second)
	power, err := s.deviceRequest(ctx, serial.SerialCommand{
		Command: command.CommandGetPower,
	})
	if err != nil {
		log.Errorf("Get power error: %s", err.Error())
		return
	}

	resp.Power.MasterPower = power[0] == 1
	return
}

func (s *PulseSerice) SetPulseParam(ctx context.Context, req *mvpulse.SetPulseParamReq) (resp *mvpulse.SetPulseParamRes, err error) {
	err = s.openGuard()
	if err != nil {
		return
	}

	resp = &mvpulse.SetPulseParamRes{}

	ctx, _ = context.WithTimeout(ctx, time.Second)

	config := req.Pulse

	if config.ExposureTick != nil {
		b := make([]byte, command.CommandSetExposure.RequestLength)
		binary.LittleEndian.PutUint16(b, uint16(config.ExposureTick.Value))
		_, err = s.deviceRequest(
			ctx,
			serial.SerialCommand{
				Command: command.CommandSetExposure,
				Arg:     b,
			},
		)
		if err != nil {
			log.Errorf("Failed to set exposure: %s", err)
			return
		}
	}

	if config.DigitalFilter != nil {
		b := make([]byte, command.CommandSetFilter.RequestLength)
		binary.LittleEndian.PutUint16(b, uint16(config.DigitalFilter.Value))
		_, err = s.deviceRequest(
			ctx,
			serial.SerialCommand{
				Command: command.CommandSetFilter,
				Arg:     b,
			},
		)
		if err != nil {
			log.Errorf("Failed to set filter: %s", err)
			return
		}
	}

	if config.PulseDelay != nil {
		b := make([]byte, command.CommandSetDelay.RequestLength)
		binary.LittleEndian.PutUint16(b, uint16(config.PulseDelay.Value))
		_, err = s.deviceRequest(
			ctx,
			serial.SerialCommand{
				Command: command.CommandSetDelay,
				Arg:     b,
			},
		)
		if err != nil {
			log.Errorf("Failed to set exposure: %s", err)
			return
		}
	}

	if config.Polarity != nil {
		var polarity uint8
		if config.Polarity.Value {
			polarity = 1
		} else {
			polarity = 0
		}
		b := make([]byte, command.CommandSetPolarity.RequestLength)
		b[0] = polarity
		_, err = s.deviceRequest(
			ctx,
			serial.SerialCommand{
				Command: command.CommandSetPolarity,
				Arg:     b,
			},
		)
		if err != nil {
			log.Errorf("Failed to set exposure: %s", err)
			return
		}
	}

	if req.Commit {
		_, err = s.deviceRequest(ctx, serial.SerialCommand{
			Command: command.CommandCommitParameters,
		})

		if err != nil {
			log.Errorf("Failed to commit parameter: %s", err.Error())
			return
		}
	}

	s.State.Config = req.Pulse
	s.State.NotifyChanged.Emit("parameter")
	return
}

func (s *PulseSerice) GetPulseParam(ctx context.Context, req *mvpulse.GetPulseParamReq) (resp *mvpulse.GetPulseParamRes, err error) {
	err = s.openGuard()
	if err != nil {
		return
	}

	resp = &mvpulse.GetPulseParamRes{
		Pulse: &mvpulse.PulseConfiguration{},
	}

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
		return
	}

	resp.Pulse.ExposureTick.Value = uint32(binary.LittleEndian.Uint16(param))

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
		return
	}

	resp.Pulse.DigitalFilter.Value = uint32(binary.LittleEndian.Uint16(param))

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
		return
	}
	resp.Pulse.PulseDelay.Value = uint32(binary.LittleEndian.Uint16(param))

	// polarity
	b = make([]byte, command.CommandGetPolarity.ResponseLength)

	param, err = s.deviceRequest(
		ctx,
		serial.SerialCommand{
			Command: command.CommandGetPolarity,
			Arg:     b,
		},
	)
	if err != nil {
		log.Errorf("Failed to get delay: %s", err)
		return
	}
	resp.Pulse.Polarity.Value = param[0] == 1

	return
}

func (s *PulseSerice) CommitParameter(ctx context.Context, req *mvpulse.CommitParameterReq) (resp *mvpulse.CommitParameterRes, err error) {
	err = s.openGuard()
	if err != nil {
		return
	}

	resp = &mvpulse.CommitParameterRes{}

	ctx, _ = context.WithTimeout(ctx, time.Second)

	_, err = s.deviceRequest(ctx, serial.SerialCommand{
		Command: command.CommandCommitParameters,
	})

	if err != nil {
		log.Errorf("Failed to commit parameter: %s", err.Error())
		return
	}
	return
}

func (s *PulseSerice) SetTriggerArm(ctx context.Context, req *mvpulse.SetTriggerArmReq) (resp *mvpulse.SetTriggerArmRes, err error) {
	err = s.openGuard()
	if err != nil {
		return
	}

	resp = &mvpulse.SetTriggerArmRes{}

	ctx, _ = context.WithTimeout(ctx, time.Second)

	var cmd command.CommandMeta
	if req.ArmTrigger {
		cmd = command.CommandArmTrigger
	} else {
		cmd = command.CommandCancelTrigger
	}

	_, err = s.deviceRequest(ctx, serial.SerialCommand{
		Command: cmd,
	})
	if err != nil {
		log.Errorf("Failed to control laser: %s", err.Error())
		return
	}

	s.State.TriggerArmed = req.ArmTrigger
	s.State.NotifyChanged.Emit("status")
	return
}

func (s *PulseSerice) GetTriggerArm(context.Context, *mvpulse.GetTriggerArmReq) (resp *mvpulse.GetTriggerArmRes, err error) {
	err = s.openGuard()
	if err != nil {
		return
	}

	resp = &mvpulse.GetTriggerArmRes{
		ArmTrigger: s.State.TriggerArmed,
	}
	return
}

func (s *PulseSerice) Reset(ctx context.Context, req *mvpulse.ResetReq) (resp *mvpulse.ResetRes, err error) {
	err = s.openGuard()
	if err != nil {
		return
	}

	resp = &mvpulse.ResetRes{}
	ctx, _ = context.WithTimeout(ctx, time.Second)

	_, err = s.deviceRequest(ctx, serial.SerialCommand{
		Command: command.CommandReset,
	})

	if err != nil {
		log.Errorf("Failed to reset: %s", err.Error())
		return
	}

	err = s.serialInstance.Disconnect()
	if err != nil {
		return
	}

	s.State.SetClosed()
	return
}

func (s *PulseSerice) Opened(context.Context, *mvpulse.OpenedReq) (resp *mvpulse.OpenedRes, err error) {
	resp = &mvpulse.OpenedRes{
		OpenedDevice: s.State.OpenedDevice,
		Opened:       s.State.Opened,
	}
	return
}

func (s *PulseSerice) ParameterStreaming(srv mvpulse.MicroVisionPulseService_ParameterStreamingServer) (err error) {

	ctx := srv.Context()
	end := make(chan interface{})
	finalize := func() {
		close(end)
	}
	defer finalize()
	go func() {
		statusChan := s.State.NotifyChanged.On("status")
		parameterChan := s.State.NotifyChanged.On("parameter")
		for {
			select {
			case <-statusChan:
				err = srv.Send(&mvpulse.ParameterStream{Opened: s.State.Opened, TriggerArmed: s.State.TriggerArmed, Power: s.State.Power, Pulse: s.State.Config})
				if err != nil {
					finalize()
				}
			case <-parameterChan:
				err = srv.Send(&mvpulse.ParameterStream{Opened: s.State.Opened, TriggerArmed: s.State.TriggerArmed, Power: s.State.Power, Pulse: s.State.Config})
				if err != nil {
					finalize()
				}
			case _, ok := <-end:
				if !ok {
					s.State.NotifyChanged.Off("status", statusChan)
					s.State.NotifyChanged.Off("parameter", parameterChan)
					return
				}
			}
		}

	}()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		req, err := srv.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		if req.Pulse != nil {
			_, err = s.SetPulseParam(ctx, &mvpulse.SetPulseParamReq{Pulse: req.Pulse, Commit: true})
			if err != nil {
				return err
			}
		}
		if req.Power != nil {
			_, err = s.SetPower(ctx, &mvpulse.SetPowerReq{Power: req.Power})
			if err != nil {
				return err
			}
		}
	}
}

func (s *PulseSerice) deviceRequest(ctx context.Context, command serial.SerialCommand) ([]byte, error) {
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

func (s *State) SetOpened(openedDevice mvpulse.SerialDevice) {
	s.OpenedDevice = &openedDevice
	s.Opened = true
	s.TriggerArmed = false
	s.Power = nil
	s.Config = nil
	s.NotifyChanged.Emit("status")
}
func (s *State) SetClosed() {
	s.OpenedDevice = nil
	s.Opened = false
	s.TriggerArmed = false
	s.Power = nil
	s.Config = nil
	s.NotifyChanged.Emit("parameter")
}
func (s *PulseSerice) openGuard() error {
	if !s.State.Opened {
		return errors.New("device not opened")
	}
	return nil
}
