package simplebridge

import (
	"fmt"
	"sync"

	"github.com/docker/docker/core"
	"github.com/docker/docker/network"
	"github.com/docker/docker/sandbox"
	"github.com/docker/docker/state"
)

type BridgeDriver struct {
	endpoints map[string]*BridgeEndpoint
	networks  map[core.DID]*BridgeNetwork
	mutex     sync.Mutex
}

func (d *BridgeDriver) endpointNames() []string {
	retval := []string{}
	d.mutex.Lock()
	defer d.mutex.Unlock()
	for key := range d.endpoints {
		retval = append(retval, key)
	}

	return retval
}

// discovery driver? should it be hooked here or in the core?
func (d *BridgeDriver) Link(s sandbox.Sandbox, id core.DID, name string, replace bool) (network.Endpoint, error) {
	ep := &BridgeEndpoint{}
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if _, ok := d.endpoints[name]; ok && !replace {
		return nil, fmt.Errorf("Endpoint %q already taken", name)
	}

	d.endpoints[name] = ep

	if err := d.createInterface(ep); err != nil { // or something
		return nil, err
	}

	return ep, nil
}

func (d *BridgeDriver) Unlink(name string) error {
	return d.destroyInterface(d.endpoints[name])
}

func (d *BridgeDriver) AddNetwork(id core.DID, s state.State) (network.Network, error) {
	bridge, err := d.createBridge(s)
	if err != nil { // use state here for parameters
		return nil, err
	}

	d.mutex.Lock()
	d.networks[id] = bridge
	d.mutex.Unlock()
	return bridge, nil
}

func (d *BridgeDriver) RemoveNetwork(id core.DID, s state.State) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	bridge, ok := d.networks[id]
	if !ok {
		return fmt.Errorf("Network %q doesn't exist", id)
	}

	return bridge.destroy(s)
}

func (d *BridgeDriver) createInterface(ep *BridgeEndpoint) error  { return nil }
func (d *BridgeDriver) destroyInterface(ep *BridgeEndpoint) error { return nil }
func (d *BridgeDriver) createBridge(s state.State) (*BridgeNetwork, error) {
	return &BridgeNetwork{driver: d}, nil
}
