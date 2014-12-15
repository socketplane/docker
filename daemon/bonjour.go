package daemon

import (
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/docker/docker/engine"
	"github.com/socketplane/bonjour"
)

const DOCKER_CLUSTER_SERVICE = "_docker._cluster"
const DOCKER_CLUSTER_SERVICE_PORT = 9999 //TODO : fix this
const DOCKER_CLUSTER_DOMAIN = "local"

func Bonjour(intfName string, cluster string, eng *engine.Engine) {
	b := bonjour.Bonjour{
		ServiceName:   DOCKER_CLUSTER_SERVICE,
		ServiceDomain: DOCKER_CLUSTER_DOMAIN,
		ServicePort:   DOCKER_CLUSTER_SERVICE_PORT,
		InterfaceName: intfName,
		BindToIntf:    true,
		Notify:        notify{eng},
	}
	b.Start()
	go staticClusterAnnounce(cluster, eng)

	select {}
}

type notify struct {
	eng *engine.Engine
}

func (n notify) NewMember(addr net.IP) {
	fmt.Println("New Member Added : ", addr)
	reportMembershipChange(n.eng, addr.String(), true)
}
func (n notify) RemoveMember(addr net.IP) {
	fmt.Println("Member Left : ", addr)
	reportMembershipChange(n.eng, addr.String(), false)
}

func reportMembershipChange(eng *engine.Engine, address string, status bool) {
	job := eng.Job("cluster_membership")
	job.Setenv("address", address)
	job.SetenvBool("added", status)
	if err := job.Run(); err != nil {
		log.Printf("Error announcing new Cluster neighbor %s : %v", address, err)
	}
}

func isMyAddress(address string) bool {
	intAddrs, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}
	for _, a := range intAddrs {
		if ipnet, ok := a.(*net.IPNet); ok && ipnet.IP.String() == address {
			return true
		}
	}
	return false
}

func staticClusterAnnounce(cluster string, eng *engine.Engine) {
	if cluster == "" {
		return
	}

	time.Sleep(time.Second * 2)
	members := strings.Split(cluster, ",")
	for _, member := range members {
		if !isMyAddress(member) {
			reportMembershipChange(eng, member, true)
		}
	}
}

func InterfaceToBind() *net.Interface {
	return bonjour.InterfaceToBind()
}
