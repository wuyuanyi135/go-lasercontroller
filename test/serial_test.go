package test

import (
	"github.com/wuyuanyi135/mvcamctrl/serial"
	"github.com/wuyuanyi135/mvcamctrl/serial/command"
	"testing"
)

func TestSerialWriteBeforeOpen(t *testing.T) {
	ser := serial.NewSerial()
	err := ser.WriteCommand(serial.SerialCommand{
		Command: command.CommandSetPower,
	})

	if err == nil {
		t.Error("Serial writing before open should fail")
	} else {
		t.Logf("Error captured (expected): %s", err.Error())
	}
}
