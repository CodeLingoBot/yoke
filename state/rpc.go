package state

import (
	"errors"
	"io"
	"net"
	"net/rpc"
	"time"
)

var (
	Timeout      = errors.New("Timeout")
	NotSupported = errors.New("not supported")
)

type (
	rpcDial struct {
		client *rpc.Client
		err    error
	}

	remoteState struct {
		timeout  time.Duration
		location string
		network  string
	}

	StateRPC struct {
		state *state
	}

	Nil struct{}
)

// ExposeRPCEndpoint starts the RPC listening server, enables remote communication with local state objects
func (local *state) ExposeRPCEndpoint(network, location string) (io.Closer, error) {
	wrap := StateRPC{
		state: local,
	}

	server := rpc.NewServer()

	if err := server.Register(&wrap); err != nil {
		return nil, err
	}

	listener, err := net.Listen(network, location)
	if err != nil {
		return nil, err
	}

	go server.Accept(listener)
	return listener, nil
}

// NewRemoteState creates and returns a State that represents a state reachable over an rpc connection
func NewRemoteState(network, location string, timeout time.Duration) State {
	remote := remoteState{
		timeout:  timeout,
		network:  network,
		location: location,
	}
	return remote
}

func call(network, location string, timeout time.Duration, method string, in interface{}, out interface{}) error {
	res := make(chan error, 1)
	go func() {
		client, err := rpc.Dial(network, location)
		if err != nil {
			res <- err
			return
		}
		defer client.Close()
		res <- client.Call(method, in, out)
	}()
	select {
	case err := <-res:
		return err
	case <-time.After(timeout):
		return Timeout
	}
}

func (c remoteState) call(method string, in interface{}, out interface{}) error {
	return call(c.network, c.location, c.timeout, method, in, out)
}

func (c remoteState) Ready() {
	for c.call("StateRPC.Ready", Nil{}, &Nil{}) != nil {
		<-time.After(time.Second)
	}
}

func (c remoteState) SetSynced(synced bool) error {
	var out bool
	return c.call("StateRPC.SetSynced", synced, &out)
}

func (c remoteState) HasSynced() (bool, error) {
	var synced bool
	err := c.call("StateRPC.HasSynced", true, &synced)
	return synced, err
}

func (c remoteState) Location() string {
	return c.location
}

func (c remoteState) GetDataDir() (string, error) {
	var dataDir string
	err := c.call("StateRPC.GetDataDir", "", &dataDir)
	return dataDir, err
}

func (c remoteState) GetRole() (string, error) {
	var role string
	err := c.call("StateRPC.GetRole", "", &role)
	return role, err
}

func (c remoteState) GetDBRole() (string, error) {
	var role string
	err := c.call("StateRPC.GetDBRole", "", &role)
	return role, err
}

func (c remoteState) SetDBRole(role string) error {
	return NotSupported
}

func (wrap *StateRPC) Ready(a Nil, b *Nil) error {
	return nil
}

func (wrap *StateRPC) GetDataDir(arg string, reply *string) error {
	*reply = wrap.state.DataDir
	return nil
}

func (wrap *StateRPC) GetRole(arg string, reply *string) error {
	*reply = wrap.state.Role
	return nil
}

func (wrap *StateRPC) GetDBRole(arg string, reply *string) error {
	*reply = wrap.state.DBRole
	return nil
}

func (wrap *StateRPC) HasSynced(arg bool, reply *bool) error {
	*reply = wrap.state.synced
	return nil
}

func (wrap *StateRPC) SetSynced(sync bool, out *bool) error {
	wrap.state.synced = sync
	return nil
}
