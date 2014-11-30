package daemon

import (
	"log"
	"net"
	"os"

	"github.com/docker/docker/engine"
	"github.com/socketplane/bonjour"
)

var dnsCache map[string]*bonjour.ServiceEntry
var queryChan chan *bonjour.ServiceEntry

const DOCKER_CLUSTER_SERVICE = "_docker._cluster"
const DOCKER_CLUSTER_SERVICE_PORT = 9999 //TODO : fix this
const DOCKER_CLUSTER_DOMAIN = "local"

func publish(ifName string) {
	var iface *net.Interface = nil
	var err error
	if ifName != "" {
		iface, err = net.InterfaceByName(ifName)
		if err != nil {
			log.Fatalln(err.Error())
		}
	}
	instance, err := os.Hostname()
	_, err = bonjour.Register(instance, DOCKER_CLUSTER_SERVICE,
		DOCKER_CLUSTER_DOMAIN, DOCKER_CLUSTER_SERVICE_PORT,
		[]string{"txtv=1", "key1=val1", "key2=val2"}, iface)
	if err != nil {
		log.Fatalln(err.Error())
	}
}

func lookup(resolver *bonjour.Resolver, query chan *bonjour.ServiceEntry) {
	for {
		select {
		case e := <-query:
			err := resolver.Lookup(e.Instance, e.Service, e.Domain)
			if err != nil {
				log.Println("Failed to browse:", err.Error())
			}
		}
	}
}

func resolve(resolver *bonjour.Resolver, results chan *bonjour.ServiceEntry, eng *engine.Engine) {
	err := resolver.Browse(DOCKER_CLUSTER_SERVICE, DOCKER_CLUSTER_DOMAIN)
	if err != nil {
		log.Println("Failed to browse:", err.Error())
	}
	for e := range results {
		if e.AddrIPv4 == nil {
			queryChan <- e
		} else if !isMyAddress(e.AddrIPv4.String()) {
			log.Printf("Cached : %s, %s, %s, %s", e.Instance, e.Service, e.Domain, e.AddrIPv4)
			dnsCache[e.AddrIPv4.String()] = e
			job := eng.Job("cluster_membership")
			job.SetenvBool("added", true)
			job.Setenv("address", e.AddrIPv4.String())
			if err := job.Run(); err != nil {
				log.Printf("Error announcing new Cluster neighbor %s : %v", e.AddrIPv4, err)
			}
		}
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

func Bonjour(intfName string, eng *engine.Engine) {
	dnsCache = make(map[string]*bonjour.ServiceEntry)
	queryChan = make(chan *bonjour.ServiceEntry)
	results := make(chan *bonjour.ServiceEntry)
	resolver, err := bonjour.NewResolver(nil, results)
	if err != nil {
		log.Println("Failed to initialize resolver:", err.Error())
		os.Exit(1)
	}

	go publish(intfName)
	go resolve(resolver, results, eng)
	go lookup(resolver, queryChan)

	select {}
}
