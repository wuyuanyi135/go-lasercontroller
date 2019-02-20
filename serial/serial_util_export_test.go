package serial

import (
	"context"
	"github.com/wuyuanyi135/mvcamctrl/serial/command"
	"testing"
	"time"
)

func TestSerial_RegisterResponse(t *testing.T) {
	s := NewSerial()
	c := make(chan []byte)
	ctx := context.Background()
	cmd := &SerialCommand{
		Command:         command.CommandVersion,
		ResponseChannel: c,
		Ctx:             ctx,
		Arg:             nil,
	}
	err := s.RegisterResponse(cmd)

	if err != nil {
		t.Fatal(err)
	}

	list := s.responseWaitingList.Load().([]*SerialCommand)
	if len(list) != 1 {
		t.Fatalf("Expected 1 item in the list but got %d", len(list))
	}

	item := list[0]
	if item != cmd {
		t.Fatalf("The command does not match")
	}
}

func TestSerial_RegisterResponseWithDelay(t *testing.T) {
	s := NewSerial()
	c := make(chan []byte)
	ctx, _ := context.WithTimeout(context.Background(), time.Second)
	cmd := &SerialCommand{
		Command:         command.CommandVersion,
		ResponseChannel: c,
		Ctx:             ctx,
		Arg:             nil,
	}
	err := s.RegisterResponse(cmd)
	if err != nil {
		t.Fatal(err)
	}
	list := s.responseWaitingList.Load().([]*SerialCommand)
	if len(list) != 1 {
		t.Fatalf("Expected 1 item in the list but got %d", len(list))
	}

	item := list[0]
	if item != cmd {
		t.Fatalf("The command does not match")
	}

	<-time.After(2 * time.Second)
	list = s.responseWaitingList.Load().([]*SerialCommand)
	if len(list) != 0 {
		t.Fatalf("Expected 0 item in the list but got %d", len(list))
	}
}
