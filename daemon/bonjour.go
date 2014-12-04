package daemon

import (
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/engine"
	"github.com/socketplane/bonjour"
)

type cacheEntry struct {
	ServiceEntry *bonjour.ServiceEntry
	LastSeen     time.Time
}

var dnsCache map[string]cacheEntry
var queryChan chan *bonjour.ServiceEntry

const DOCKER_CLUSTER_SERVICE = "_docker._cluster"
const DOCKER_CLUSTER_SERVICE_PORT = 9999 //TODO : fix this
const DOCKER_CLUSTER_DOMAIN = "local"

func publish(ifName string) {
	sleeper := time.Second * 30
	for {
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
		time.Sleep(sleeper)
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
			if e.TTL > 0 {
				if _, ok := dnsCache[e.AddrIPv4.String()]; !ok {
					log.Printf("New Member : %s, %s, %s, %s",
						e.Instance, e.Service, e.Domain, e.AddrIPv4)
					reportMembershipChange(eng, e.AddrIPv4.String(), true)
				}
				dnsCache[e.AddrIPv4.String()] = cacheEntry{e, time.Now()}
			} else {
				log.Printf("Member Gone : %s, %s, %s, %s", e.Instance, e.Service, e.Domain, e.AddrIPv4)
				reportMembershipChange(eng, e.AddrIPv4.String(), false)
				delete(dnsCache, e.AddrIPv4.String())
			}
		}
	}
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

func keepAlive(resolver *bonjour.Resolver, eng *engine.Engine) {
	sleeper := time.Second * 30
	for {
		for key, e := range dnsCache {
			if time.Now().Sub(e.LastSeen) > sleeper*2 {
				reportMembershipChange(eng, key, false)
				delete(dnsCache, key)
				log.Println("Member timed out : ", key)
			}
		}
		time.Sleep(sleeper)
	}
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

func Bonjour(intfName string, cluster string, eng *engine.Engine) {
	dnsCache = make(map[string]cacheEntry)
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
	go keepAlive(resolver, eng)
	go staticClusterAnnounce(cluster, eng)

	select {}
}
