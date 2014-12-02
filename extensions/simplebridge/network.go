package simplebridge

import (
	"path"

	"github.com/docker/docker/core"
	"github.com/docker/docker/network"
	"github.com/docker/docker/sandbox"

	"github.com/vishvananda/netlink"
)

type BridgeNetwork struct {
	bridge *netlink.Bridge
	ID     string
	driver *BridgeDriver
}

func (b *BridgeNetwork) Driver() network.Driver {
	return b.driver
}

func (b *BridgeNetwork) Id() core.DID {
	return core.DID(b.ID)
}

func (b *BridgeNetwork) List() []string {
	return b.driver.endpointNames()
}

func (b *BridgeNetwork) Link(s sandbox.Sandbox, name string, replace bool) (network.Endpoint, error) {
	return b.driver.Link(b.ID, name, s, replace)
}

func (b *BridgeNetwork) Unlink(name string) error {
	return b.driver.Unlink(b.ID, name, nil)
}

func (b *BridgeNetwork) destroy() error {
	if err := b.driver.state.Remove(path.Join("networks", b.ID)); err != nil {
		return err
	}

	return netlink.LinkDel(b.bridge)
}
