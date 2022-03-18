package server

type State string

var States = struct {
	Running  State
	Stopping State
	Stopped  State
}{
	Running:  "running",
	Stopping: "stopping",
	Stopped:  "stopped",
}
