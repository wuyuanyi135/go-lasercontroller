package serial

import (
	"context"
	"github.com/wuyuanyi135/lasercontroller/serial/command"
)

type SerialCommand struct {
	Command         command.CommandMeta
	Arg             []byte
	ResponseChannel chan []byte
	Ctx             context.Context
}
