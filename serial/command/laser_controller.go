package command

type Command byte

const (
	COMMAND_VERSION_0_2 Command = 0x20
	COMMAND_RESET_0_0   Command = 0x35

	COMMAND_ARM_TRIGGER_0_0    Command = 0x40
	COMMAND_CANCEL_TRIGGER_0_0 Command = 0x41

	COMMAND_SET_FILTER_2_0 Command = 0x42
	COMMAND_GET_FILTER_0_2 Command = 0x43

	COMMAND_SET_EXPOSURE_2_0 Command = 0x44
	COMMAND_GET_EXPOSURE_0_2 Command = 0x45

	COMMAND_SET_DELAY_2_0 Command = 0x46
	COMMAND_GET_DELAY_0_2 Command = 0x47

	COMMAND_COMMIT_PARAMETERS_0_0 Command = 0x50
	COMMAND_SET_POWER_1_0         Command = 0x30
	COMMAND_GET_POWER_0_1         Command = 0x31
)

type CommandMeta struct {
	Command        Command
	RequestLength  int
	ResponseLength int
}

var CommandVersion = CommandMeta{Command: COMMAND_VERSION_0_2, RequestLength: 0, ResponseLength: 2}
var CommandReset = CommandMeta{Command: COMMAND_RESET_0_0, RequestLength: 0, ResponseLength: 0}
var CommandArmTrigger = CommandMeta{Command: COMMAND_ARM_TRIGGER_0_0, RequestLength: 0, ResponseLength: 0}
var CommandCancelTrigger = CommandMeta{Command: COMMAND_CANCEL_TRIGGER_0_0, RequestLength: 0, ResponseLength: 0}
var CommandSetFilter = CommandMeta{Command: COMMAND_SET_FILTER_2_0, RequestLength: 2, ResponseLength: 0}
var CommandGetFilter = CommandMeta{Command: COMMAND_GET_FILTER_0_2, RequestLength: 0, ResponseLength: 2}
var CommandSetExposure = CommandMeta{Command: COMMAND_SET_EXPOSURE_2_0, RequestLength: 2, ResponseLength: 0}
var CommandGetExposure = CommandMeta{Command: COMMAND_GET_EXPOSURE_0_2, RequestLength: 0, ResponseLength: 2}
var CommandSetDelay = CommandMeta{Command: COMMAND_SET_DELAY_2_0, RequestLength: 2, ResponseLength: 0}
var CommandGetDelay = CommandMeta{Command: COMMAND_GET_DELAY_0_2, RequestLength: 0, ResponseLength: 2}
var CommandCommitParameters = CommandMeta{Command: COMMAND_COMMIT_PARAMETERS_0_0, RequestLength: 0, ResponseLength: 0}
var CommandSetPower = CommandMeta{Command: COMMAND_SET_POWER_1_0, RequestLength: 1, ResponseLength: 0}
var CommandGetPower = CommandMeta{Command: COMMAND_GET_POWER_0_1, RequestLength: 0, ResponseLength: 1}
