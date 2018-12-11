package serial

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/op/go-logging"
	"go.bug.st/serial.v1"
	"io/ioutil"
	"os"
	"path"
	"sync/atomic"
	"time"
)

const (
	ErrorChannelClosed = "Receiving channel closed. Remove the handler"
	DefaultBaudRate    = 921600
	DefaultDataBits    = 8
	DirPath            = "/dev/serial/by-id"
	ReceiveBufferSize  = 128
)

type Serial struct {
	instance serial.Port
	baudRate int
	dataBits int
	stopBits serial.StopBits
	parity   serial.Parity

	responseWaitingList *atomic.Value
	serialReceiveChan   chan byte
}

var log = logging.MustGetLogger("Serial")

func NewSerial() Serial {
	responseWaitingList := &atomic.Value{}
	responseWaitingList.Store([]*SerialCommand{})
	return Serial{
		instance: nil,
		baudRate: DefaultBaudRate,
		dataBits: DefaultDataBits,
		stopBits: serial.OneStopBit,
		parity:   serial.NoParity,

		responseWaitingList: responseWaitingList,
		serialReceiveChan:   make(chan byte, ReceiveBufferSize),
	}
}
func ListSerialPorts() (map[string]string, error) {
	dirPath := DirPath
	infos, e := ioutil.ReadDir(dirPath)
	if e != nil {
		errMessage := fmt.Sprintf("Failed to list serial devices in %s", dirPath)
		return nil, errors.New(errMessage)
	}

	mapping := map[string]string{}

	for _, port := range infos {
		destination, e := os.Readlink(path.Join(dirPath, port.Name()))
		if e != nil {
			continue
		}
		mapping[port.Name()] = path.Join(dirPath, destination)
	}
	return mapping, nil
}

func (s *Serial) ConnectByName(name string) error {
	mapping, err := ListSerialPorts()

	if err != nil {
		errMsg := fmt.Sprintf("Failed to list port: " + err.Error())
		return errors.New(errMsg)
	}

	return s.ConnectByPath(mapping[name])
}

func (s *Serial) ConnectByPath(p string) error {
	if s.instance != nil {
		errMsg := fmt.Sprintf("Failed to connect by path: %s is already opened", p)
		return errors.New(errMsg)
	}

	var err error
	s.instance, err = serial.Open(p, &serial.Mode{
		BaudRate: s.baudRate,
		DataBits: s.dataBits,
		Parity:   s.parity,
		StopBits: s.stopBits,
	})
	if err != nil {
		errMsg := fmt.Sprintf("failed to connect by path: %s: %s", p, err.Error())
		s.instance = nil
		return errors.New(errMsg)
	}
	// start serial receive listener
	go s.serialReceiver()
	go s.responseHandler()

	//err=s.instance.ResetInputBuffer()
	//if err != nil {
	//	errMsg := fmt.Sprintf("failed to get modem bits: %s", err.Error())
	//	s.instance = nil
	//	return errors.New(errMsg)
	//}
	// TODO: after connection it seems the message cannot be sent immediately?
	//for {
	//	bits, err := s.instance.GetModemStatusBits()
	//	if err != nil {
	//		errMsg := fmt.Sprintf("failed to get modem bits: %s", err.Error())
	//		s.instance = nil
	//		return errors.New(errMsg)
	//	}
	//
	//	if bits.CTS == false {
	//		break
	//	}
	//
	//	time.Sleep(50*time.Millisecond)
	//}
	time.Sleep(500 * time.Millisecond)
	return nil
}

func (s *Serial) Disconnect() error {
	if s.instance == nil {
		return nil
	}

	err := s.instance.Close()
	return err
}

func (s *Serial) WriteCommand(cmd SerialCommand) error {
	if s.instance == nil {
		return errors.New("port is not open")
	}

	var packet bytes.Buffer
	packet.WriteByte(byte(cmd.Command.Command))
	packet.Write(cmd.Arg)

	_, err := s.instance.Write(packet.Bytes())
	return err
}

func (s *Serial) RegisterResponse(cmd *SerialCommand) error {
	if cmd.ResponseChannel == nil {
		return errors.New("response channel is not initialized")
	}

	list, _ := s.responseWaitingList.Load().([]*SerialCommand)
	list = append(list, cmd)
	s.responseWaitingList.Store(list)

	// Move timeout handling to serial handler
	//if _, ok := cmd.Ctx.Deadline(); ok == true {
	//	go func() {
	//		done := cmd.Ctx.Done()
	//
	//		if done == nil {
	//			log.Warning("Unable to cancel the context: %#v", cmd)
	//			return
	//		}
	//
	//		<-done
	//
	//		log.Warning("Timeout handling %#v", cmd)
	//		err := s.UnregisterExactly(cmd)
	//		if err != nil {
	//			log.Errorf("Failed to unregister cmd %#v: %s", cmd, err.Error())
	//			return
	//		}
	//
	//	}()
	//}
	return nil
}

func (s *Serial) UnregisterExactly(cmd *SerialCommand) error {
	list := s.responseWaitingList.Load().([]*SerialCommand)
	for i, v := range list {
		if v == cmd {
			list = append(list[:i], list[i+1:]...)
			s.responseWaitingList.Store(list)
			return nil
		}
	}
	return errors.New("Command not found in the queue")
}

func (s *Serial) serialReceiver() {
	if s.instance == nil {
		log.Error("Can not receive from serial port: port is not opened")
		return
	}
	if s.serialReceiveChan == nil {
		log.Error("Channel is not initialized")
		return
	}

	// clean up when the function exits
	defer close(s.serialReceiveChan)

	var recvBuf = make([]byte, 128)
	for {
		n, err := s.instance.Read(recvBuf)
		if err != nil {
			log.Warning("Serial instance has been removed. Unregister the handler.")
			return
		}

		for i := 0; i < n; i++ {
			s.serialReceiveChan <- recvBuf[i]
		}
	}
}

func (s *Serial) timeoutChannels() <-chan *SerialCommand {
	commands := s.responseWaitingList.Load().([]*SerialCommand)
	timeoutChan := make(chan *SerialCommand)
	for _, cmd := range commands {
		_, ok := cmd.Ctx.Deadline()
		if ok {
			// has deadline
			go func() {
				<-cmd.Ctx.Done()
				timeoutChan <- cmd
			}()
		}
	}
	return timeoutChan
}

// Coroutine function that receive the responses and dispatch them. It should be registered when a port is successfully
// opened. The handler is deactivated when the port is closed
func (s *Serial) responseHandler() {
	var pendingCommand *SerialCommand
OUTER:
	for {
		// process the received bytes
		if s.serialReceiveChan == nil {
			log.Error("Serial receive channel is not initialized")
			return
		}

		timeoutChannels := s.timeoutChannels()
		var cmd byte
		var ok bool
		select {
		case cmd, ok = <-s.serialReceiveChan:
			if !ok {
				// channel closed
				log.Info("Serial receiver channel closed.")
				return
			}
		// continue to process the command

		case timeoutChannel := <-timeoutChannels:
			// channel timed out, remove it. It does not affect the channel receiving arguments
			log.Warning("Timeout handling %#v", cmd)

			err := s.UnregisterExactly(timeoutChannel)
			if err != nil {
				log.Errorf("Failed to unregister cmd %#v: %s", cmd, err.Error())
				return
			}
			continue OUTER
		}

		// looking for the pending command for resolving
		list := s.responseWaitingList.Load().([]*SerialCommand)
		for _, pendingCommand = range list {
			if byte(pendingCommand.Command.Command) == cmd {

				// wait for the required arguments fulfilled
				if pendingCommand.Command.ResponseLength != 0 {
					responseBuffer := make([]byte, pendingCommand.Command.ResponseLength)
					for i := 0; i < pendingCommand.Command.ResponseLength; i++ {
						arg, ok := <-s.serialReceiveChan
						if !ok {
							log.Info(ErrorChannelClosed)
							return
						}
						responseBuffer[i] = arg
					}

					if pendingCommand.ResponseChannel != nil {
						pendingCommand.ResponseChannel <- responseBuffer
						close(pendingCommand.ResponseChannel)
					}

				} else {
					// this command does not require response argument. Just notify the channel if it exists
					if pendingCommand.ResponseChannel != nil {
						pendingCommand.ResponseChannel <- nil
						close(pendingCommand.ResponseChannel)
					}
				}

				// remove the command from the pending position
				err := s.UnregisterExactly(pendingCommand)
				if err != nil {
					return
				}

				continue OUTER
			}
		}
		// should not reach here
		log.Warningf("Unresolved command: %#v", pendingCommand)
	}
}

// shortcut for writing command and register response handler
func (s *Serial) WriteCommandAndRegisterResponse(cmd SerialCommand) error {
	err := s.RegisterResponse(&cmd)
	if err != nil {
		log.Errorf("Failed to register response: %s", err.Error())
		return err
	}

	err = s.WriteCommand(cmd)
	if err != nil {
		log.Errorf("Failed to write command: %s", err.Error())
		_ = s.UnregisterExactly(&cmd)
		return err
	}

	return nil
}
