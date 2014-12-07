package bonjour

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/socketplane/bonjour/Godeps/_workspace/src/github.com/miekg/dns"
	"github.com/socketplane/bonjour/Godeps/_workspace/src/golang.org/x/net/ipv4"
	"github.com/socketplane/bonjour/Godeps/_workspace/src/golang.org/x/net/ipv6"
)

var (
	// Multicast groups used by mDNS
	mdnsGroupIPv4 = net.IPv4(224, 0, 0, 251)
	mdnsGroupIPv6 = net.ParseIP("ff02::fb")

	// mDNS wildcard addresses
	mdnsWildcardAddrIPv4 = &net.UDPAddr{
		IP:   net.ParseIP("224.0.0.0"),
		Port: 5353,
	}
	mdnsWildcardAddrIPv6 = &net.UDPAddr{
		IP:   net.ParseIP("[ff02::]"),
		Port: 5353,
	}

	// mDNS endpoint addresses
	ipv4Addr = &net.UDPAddr{
		IP:   mdnsGroupIPv4,
		Port: 5353,
	}
	ipv6Addr = &net.UDPAddr{
		IP:   mdnsGroupIPv6,
		Port: 5353,
	}
)

// Register a service by given arguments. This call will take the system's hostname
// and lookup IP by that hostname.
func Register(instance, service, domain string, port int, text []string, iface *net.Interface) (chan<- bool, error) {
	entry := NewServiceEntry(instance, service, domain)
	entry.Port = port
	entry.Text = text

	if entry.Instance == "" {
		return nil, fmt.Errorf("Missing service instance name")
	}
	if entry.Service == "" {
		return nil, fmt.Errorf("Missing service name")
	}
	if entry.Domain == "" {
		entry.Domain = "local"
	}
	if entry.Port == 0 {
		return nil, fmt.Errorf("Missing port")
	}

	var err error
	if entry.HostName == "" {
		entry.HostName, err = os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("Could not determine host")
		}
	}
	entry.HostName = fmt.Sprintf("%s.", trimDot(entry.HostName))

	if iface == nil {
		addrs, err := net.LookupIP(entry.HostName)
		if err != nil {
			// Try appending the host domain suffix and lookup again
			// (required for Linux-based hosts)
			tmpHostName := fmt.Sprintf("%s%s.", entry.HostName, entry.Domain)
			addrs, err = net.LookupIP(tmpHostName)
			if err != nil {
				intAddrs, err := net.InterfaceAddrs()
				if err != nil {
					return nil, err
				}
				for _, a := range intAddrs {
					if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
						addrs = append(addrs, ipnet.IP)
					}
				}
				if len(addrs) == 0 {
					return nil, fmt.Errorf("Could not determine host IP addresses for %s", entry.HostName)
				}
			}
		}
		for i := 0; i < len(addrs); i++ {
			if ipv4 := addrs[i].To4(); ipv4 != nil {
				entry.AddrIPv4 = addrs[i]
			} else if ipv6 := addrs[i].To16(); ipv6 != nil {
				entry.AddrIPv6 = addrs[i]
			}
		}
	} else {
		addrs, err := iface.Addrs()
		if err == nil {
			for i := 0; i < len(addrs); i++ {
				addr := addrs[i].String()
				addr = strings.Split(addr, "/")[0]
				ip := net.ParseIP(addr)
				if ip != nil {
					if ip.To4() != nil {
						entry.AddrIPv4 = ip
					} else if ip.To16() != nil {
						entry.AddrIPv6 = ip
					}
				}
			}
		}
	}

	s, err := newServer(iface)
	if err != nil {
		return nil, err
	}

	s.service = entry
	go s.mainloop()
	go s.probe()

	return s.shutdownCh, nil
}

// Register a service proxy by given argument. This call will skip the hostname/IP lookup and
// will use the provided values.
func RegisterProxy(instance, service, domain string, port int, host, ip string, text []string, iface *net.Interface) (chan<- bool, error) {
	entry := NewServiceEntry(instance, service, domain)
	entry.Port = port
	entry.Text = text
	entry.HostName = host

	if entry.Instance == "" {
		return nil, fmt.Errorf("Missing service instance name")
	}
	if entry.Service == "" {
		return nil, fmt.Errorf("Missing service name")
	}
	if entry.HostName == "" {
		return nil, fmt.Errorf("Missing host name")
	}
	if entry.Domain == "" {
		entry.Domain = "local"
	}
	if entry.Port == 0 {
		return nil, fmt.Errorf("Missing port")
	}

	if !strings.HasSuffix(trimDot(entry.HostName), entry.Domain) {
		entry.HostName = fmt.Sprintf("%s.%s.", trimDot(entry.HostName), trimDot(entry.Domain))
	}

	ipAddr := net.ParseIP(ip)
	if ipAddr == nil {
		return nil, fmt.Errorf("Failed to parse given IP: %v", ip)
	} else if ipv4 := ipAddr.To4(); ipv4 != nil {
		entry.AddrIPv4 = ipAddr
	} else if ipv6 := ipAddr.To16(); ipv6 != nil {
		entry.AddrIPv4 = ipAddr
	} else {
		return nil, fmt.Errorf("The IP is neither IPv4 nor IPv6: %#v", ipAddr)
	}

	s, err := newServer(iface)
	if err != nil {
		return nil, err
	}

	s.service = entry
	go s.mainloop()
	go s.probe()

	return s.shutdownCh, nil
}

// Server structure incapsulates both IPv4/IPv6 UDP connections
type server struct {
	service        *ServiceEntry
	ipv4conn       *net.UDPConn
	ipv6conn       *net.UDPConn
	shouldShutdown bool
	shutdownCh     chan bool
	shutdownLock   sync.Mutex
}

// Constructs server structure
func newServer(iface *net.Interface) (*server, error) {
	// Create wildcard connections (because :5353 can be already taken by other apps)
	ipv4conn, err := net.ListenMulticastUDP("udp4", iface, mdnsWildcardAddrIPv4)
	if err != nil {
		log.Printf("[ERR] bonjour: Failed to bind to udp4 port: %v", err)
	}
	ipv6conn, err := net.ListenMulticastUDP("udp6", iface, mdnsWildcardAddrIPv6)
	if err != nil {
		log.Printf("[ERR] bonjour: Failed to bind to udp6 port: %v", err)
	}
	if ipv4conn == nil && ipv6conn == nil {
		return nil, fmt.Errorf("[ERR] bonjour: Failed to bind to any udp port!")
	}

	// Join multicast groups to receive announcements
	p1 := ipv4.NewPacketConn(ipv4conn)
	p2 := ipv6.NewPacketConn(ipv6conn)
	if iface != nil {
		errCount := 0
		if err := p1.JoinGroup(iface, &net.UDPAddr{IP: mdnsGroupIPv4}); err != nil {
			errCount++
		}
		if err := p2.JoinGroup(iface, &net.UDPAddr{IP: mdnsGroupIPv6}); err != nil {
			errCount++
		}
		if errCount == 2 {
			return nil, fmt.Errorf("Failed to join multicast group on both v4 and v6")
		}
	} else {
		ifaces, err := net.Interfaces()
		if err != nil {
			return nil, err
		}
		errCount1, errCount2 := 0, 0
		for _, iface := range ifaces {
			if err := p1.JoinGroup(&iface, &net.UDPAddr{IP: mdnsGroupIPv4}); err != nil {
				errCount1++
			}
			if err := p2.JoinGroup(&iface, &net.UDPAddr{IP: mdnsGroupIPv6}); err != nil {
				errCount2++
			}
		}
		if len(ifaces) == errCount1 && len(ifaces) == errCount2 {
			return nil, fmt.Errorf("Failed to join multicast group on all interfaces!")
		}
	}

	s := &server{
		ipv4conn:   ipv4conn,
		ipv6conn:   ipv6conn,
		shutdownCh: make(chan bool),
	}

	return s, nil
}

// Start listeners and waits for the shutdown signal from exit channel
func (s *server) mainloop() {
	if s.ipv4conn != nil {
		go s.recv(s.ipv4conn)
	}
	if s.ipv6conn != nil {
		go s.recv(s.ipv6conn)
	}
	for !s.shouldShutdown {
		select {
		case <-s.shutdownCh:
			s.shutdown()
		}
	}
}

// Shutdown server will close currently open connections & channel
func (s *server) shutdown() error {
	s.shutdownLock.Lock()
	defer s.shutdownLock.Unlock()

	s.unregister()

	if s.shouldShutdown {
		return nil
	}
	s.shouldShutdown = true
	close(s.shutdownCh)

	if s.ipv4conn != nil {
		s.ipv4conn.Close()
	}
	if s.ipv6conn != nil {
		s.ipv6conn.Close()
	}
	return nil
}

// recv is a long running routine to receive packets from an interface
func (s *server) recv(c *net.UDPConn) {
	if c == nil {
		return
	}
	buf := make([]byte, 65536)
	for !s.shouldShutdown {
		n, from, err := c.ReadFrom(buf)
		if err != nil {
			continue
		}
		if err := s.parsePacket(buf[:n], from); err != nil {
			log.Printf("[ERR] bonjour: Failed to handle query: %v", err)
		}
	}
}

// parsePacket is used to parse an incoming packet
func (s *server) parsePacket(packet []byte, from net.Addr) error {
	var msg dns.Msg
	if err := msg.Unpack(packet); err != nil {
		log.Printf("[ERR] bonjour: Failed to unpack packet: %v", err)
		return err
	}
	return s.handleQuery(&msg, from)
}

// handleQuery is used to handle an incoming query
func (s *server) handleQuery(query *dns.Msg, from net.Addr) error {
	// Ignore answer for now
	if len(query.Answer) > 0 {
		return nil
	}
	// Ignore questions with Authorative section for now
	if len(query.Ns) > 0 {
		return nil
	}

	// Handle each question
	var (
		resp dns.Msg
		err  error
	)
	if len(query.Question) > 0 {
		for i, _ := range query.Question {
			resp = dns.Msg{}
			resp.SetReply(query)
			resp.Answer = []dns.RR{}
			resp.Extra = []dns.RR{}
			if err = s.handleQuestion(query.Question[i], &resp); err != nil {
				log.Printf("[ERR] bonjour: failed to handle question %v: %v",
					query.Question[i], err)
				continue
			}
			// Check if there is an answer
			if len(resp.Answer) > 0 {
				//return s.sendResponse(&resp, from)
				//log.Println("====== BEGIN ======")
				//log.Println(resp.String())
				//log.Println("======= END =======")
				if e := s.multicastResponse(&resp); e != nil {
					err = e
				}
			}
		}
	}

	return err
}

// handleQuestion is used to handle an incoming question
func (s *server) handleQuestion(q dns.Question, resp *dns.Msg) error {
	if s.service == nil {
		return nil
	}

	switch q.Name {
	case s.service.ServiceName():
		s.composeBrowsingAnswers(resp, 3200)
	case s.service.ServiceInstanceName():
		s.composeLookupAnswers(resp, 3200)
	}

	return nil
}

func (s *server) composeBrowsingAnswers(resp *dns.Msg, ttl uint32) {
	ptr := &dns.PTR{
		Hdr: dns.RR_Header{
			Name:   s.service.ServiceName(),
			Rrtype: dns.TypePTR,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		Ptr: s.service.ServiceInstanceName(),
	}
	resp.Answer = append(resp.Answer, ptr)

	txt := &dns.TXT{
		Hdr: dns.RR_Header{
			Name:   s.service.ServiceInstanceName(),
			Rrtype: dns.TypeTXT,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		Txt: s.service.Text,
	}
	srv := &dns.SRV{
		Hdr: dns.RR_Header{
			Name:   s.service.ServiceInstanceName(),
			Rrtype: dns.TypeSRV,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		Priority: 0,
		Weight:   0,
		Port:     uint16(s.service.Port),
		Target:   s.service.HostName,
	}
	resp.Extra = append(resp.Extra, srv, txt)

	if s.service.AddrIPv4 != nil {
		a := &dns.A{
			Hdr: dns.RR_Header{
				Name:   s.service.HostName,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    ttl,
			},
			A: s.service.AddrIPv4,
		}
		resp.Extra = append(resp.Extra, a)
	}
	if s.service.AddrIPv6 != nil {
		aaaa := &dns.AAAA{
			Hdr: dns.RR_Header{
				Name:   s.service.HostName,
				Rrtype: dns.TypeAAAA,
				Class:  dns.ClassINET,
				Ttl:    ttl,
			},
			AAAA: s.service.AddrIPv6,
		}
		resp.Extra = append(resp.Extra, aaaa)
	}
}

func (s *server) composeLookupAnswers(resp *dns.Msg, ttl uint32) {
	ptr := &dns.PTR{
		Hdr: dns.RR_Header{
			Name:   s.service.ServiceName(),
			Rrtype: dns.TypePTR,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		Ptr: s.service.ServiceInstanceName(),
	}
	srv := &dns.SRV{
		Hdr: dns.RR_Header{
			Name:   s.service.ServiceInstanceName(),
			Rrtype: dns.TypeSRV,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		Priority: 0,
		Weight:   0,
		Port:     uint16(s.service.Port),
		Target:   s.service.HostName,
	}
	txt := &dns.TXT{
		Hdr: dns.RR_Header{
			Name:   s.service.ServiceInstanceName(),
			Rrtype: dns.TypeTXT,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		Txt: s.service.Text,
	}
	dnssd := &dns.PTR{
		Hdr: dns.RR_Header{
			Name:   fmt.Sprintf("_services._dns-sd._udp.%s.", trimDot(s.service.Domain)),
			Rrtype: dns.TypePTR,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		Ptr: s.service.ServiceName(),
	}
	resp.Answer = append(resp.Answer, srv, txt, ptr, dnssd)

	if s.service.AddrIPv4 != nil {
		a := &dns.A{
			Hdr: dns.RR_Header{
				Name:   s.service.HostName,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    120,
			},
			A: s.service.AddrIPv4,
		}
		resp.Extra = append(resp.Extra, a)
	}
	if s.service.AddrIPv6 != nil {
		aaaa := &dns.AAAA{
			Hdr: dns.RR_Header{
				Name:   s.service.HostName,
				Rrtype: dns.TypeAAAA,
				Class:  dns.ClassINET,
				Ttl:    120,
			},
			AAAA: s.service.AddrIPv6,
		}
		resp.Extra = append(resp.Extra, aaaa)
	}
}

// Perform probing & announcement
//TODO: implement a proper probing & conflict resolution
func (s *server) probe() {
	q := new(dns.Msg)
	q.SetQuestion(s.service.ServiceInstanceName(), dns.TypePTR)
	q.RecursionDesired = false

	srv := &dns.SRV{
		Hdr: dns.RR_Header{
			Name:   s.service.ServiceInstanceName(),
			Rrtype: dns.TypeSRV,
			Class:  dns.ClassINET,
			Ttl:    3200,
		},
		Priority: 0,
		Weight:   0,
		Port:     uint16(s.service.Port),
		Target:   s.service.HostName,
	}
	txt := &dns.TXT{
		Hdr: dns.RR_Header{
			Name:   s.service.ServiceInstanceName(),
			Rrtype: dns.TypeTXT,
			Class:  dns.ClassINET,
			Ttl:    3200,
		},
		Txt: s.service.Text,
	}
	q.Ns = []dns.RR{srv, txt}

	randomizer := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < 3; i++ {
		if err := s.multicastResponse(q); err != nil {
			log.Println("[ERR] bonjour: failed to send probe:", err.Error())
		}
		time.Sleep(time.Duration(randomizer.Intn(250)) * time.Millisecond)
	}

	resp := new(dns.Msg)
	resp.Answer = []dns.RR{}
	resp.Extra = []dns.RR{}
	s.composeLookupAnswers(resp, 3200)
	for i := 0; i < 3; i++ {
		if err := s.multicastResponse(resp); err != nil {
			log.Println("[ERR] bonjour: failed to send announcement:", err.Error())
		}
		time.Sleep(2 * time.Second)
	}
}

func (s *server) unregister() error {
	resp := new(dns.Msg)
	resp.Answer = []dns.RR{}
	resp.Extra = []dns.RR{}
	s.composeLookupAnswers(resp, 0)
	return s.multicastResponse(resp)
}

// sendResponse is used to send a unicast response packet
func (s *server) sendResponse(resp *dns.Msg, from net.Addr) error {
	buf, err := resp.Pack()
	if err != nil {
		return err
	}
	addr := from.(*net.UDPAddr)
	if addr.IP.To4() != nil {
		_, err = s.ipv4conn.WriteToUDP(buf, addr)
		return err
	} else {
		_, err = s.ipv6conn.WriteToUDP(buf, addr)
		return err
	}
}

// multicastResponse us used to send a multicast response packet
func (c *server) multicastResponse(msg *dns.Msg) error {
	buf, err := msg.Pack()
	if err != nil {
		log.Println("Failed to pack message!")
		return err
	}
	if c.ipv4conn != nil {
		c.ipv4conn.WriteTo(buf, ipv4Addr)
	}
	if c.ipv6conn != nil {
		c.ipv6conn.WriteTo(buf, ipv6Addr)
	}
	return nil
}
