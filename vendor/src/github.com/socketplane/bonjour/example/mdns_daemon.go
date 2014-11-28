package main

import (
	"log"
	"net"
	"os"

	"github.com/socketplane/bonjour"
)

var dnsCache map[string]*bonjour.ServiceEntry
var queryChan chan *bonjour.ServiceEntry

const DOCKER_CLUSTER_SERVICE = "_foobar._service"
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
	_, err = bonjour.Register(instance, DOCKER_CLUSTER_SERVICE, DOCKER_CLUSTER_DOMAIN, 9999, []string{"txtv=1", "app=test"}, iface)
	if err != nil {
		log.Fatalln(err.Error())
	}
}

func lookup(resolver *bonjour.Resolver, query chan *bonjour.ServiceEntry) {
	for {
		select {
		case e := <-query:
			log.Printf("lookup : %s, %s, %s, %s", e.Instance, e.Service, e.Domain, e.AddrIPv4)
			err := resolver.Lookup(e.Instance, e.Service, e.Domain)
			if err != nil {
				log.Println("Failed to browse:", err.Error())
			}
		}
	}
}

func resolve(resolver *bonjour.Resolver, results chan *bonjour.ServiceEntry) {
	err := resolver.Browse(DOCKER_CLUSTER_SERVICE, DOCKER_CLUSTER_DOMAIN)
	if err != nil {
		log.Println("Failed to browse:", err.Error())
	}
	for e := range results {
		if e.AddrIPv4 == nil {
			queryChan <- e
		} else {
			log.Printf("Cached : %s, %s, %s, %s", e.Instance, e.Service, e.Domain, e.AddrIPv4)
			dnsCache[e.AddrIPv4.String()] = e
		}
	}
}

func main() {
	dnsCache = make(map[string]*bonjour.ServiceEntry)
	queryChan = make(chan *bonjour.ServiceEntry)
	results := make(chan *bonjour.ServiceEntry)
	resolver, err := bonjour.NewResolver(nil, results)
	if err != nil {
		log.Println("Failed to initialize resolver:", err.Error())
		os.Exit(1)
	}

	var intfName = ""
	if len(os.Args) > 1 {
		intfName = os.Args[1]
	}
	go publish(intfName)
	go resolve(resolver, results)
	go lookup(resolver, queryChan)

	select {}
}
