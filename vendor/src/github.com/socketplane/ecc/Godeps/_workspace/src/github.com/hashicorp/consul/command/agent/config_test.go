package agent

import (
	"bytes"
	"encoding/base64"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestConfigEncryptBytes(t *testing.T) {
	// Test with some input
	src := []byte("abc")
	c := &Config{
		EncryptKey: base64.StdEncoding.EncodeToString(src),
	}

	result, err := c.EncryptBytes()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if !bytes.Equal(src, result) {
		t.Fatalf("bad: %#v", result)
	}

	// Test with no input
	c = &Config{}
	result, err = c.EncryptBytes()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if len(result) > 0 {
		t.Fatalf("bad: %#v", result)
	}
}

func TestDecodeConfig(t *testing.T) {
	// Basics
	input := `{"data_dir": "/tmp/", "log_level": "debug"}`
	config, err := DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.DataDir != "/tmp/" {
		t.Fatalf("bad: %#v", config)
	}

	if config.LogLevel != "debug" {
		t.Fatalf("bad: %#v", config)
	}

	// Without a protocol
	input = `{"node_name": "foo", "datacenter": "dc2"}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.NodeName != "foo" {
		t.Fatalf("bad: %#v", config)
	}

	if config.Datacenter != "dc2" {
		t.Fatalf("bad: %#v", config)
	}

	if config.SkipLeaveOnInt != DefaultConfig().SkipLeaveOnInt {
		t.Fatalf("bad: %#v", config)
	}

	if config.LeaveOnTerm != DefaultConfig().LeaveOnTerm {
		t.Fatalf("bad: %#v", config)
	}

	// Server bootstrap
	input = `{"server": true, "bootstrap": true}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if !config.Server {
		t.Fatalf("bad: %#v", config)
	}

	if !config.Bootstrap {
		t.Fatalf("bad: %#v", config)
	}

	// Expect bootstrap
	input = `{"server": true, "bootstrap_expect": 3}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if !config.Server {
		t.Fatalf("bad: %#v", config)
	}

	if config.BootstrapExpect != 3 {
		t.Fatalf("bad: %#v", config)
	}

	// DNS setup
	input = `{"ports": {"dns": 8500}, "recursor": "8.8.8.8", "domain": "foobar"}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.Ports.DNS != 8500 {
		t.Fatalf("bad: %#v", config)
	}

	if config.DNSRecursor != "8.8.8.8" {
		t.Fatalf("bad: %#v", config)
	}

	if config.Domain != "foobar" {
		t.Fatalf("bad: %#v", config)
	}

	// RPC configs
	input = `{"ports": {"http": 1234, "rpc": 8100}, "client_addr": "0.0.0.0"}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.ClientAddr != "0.0.0.0" {
		t.Fatalf("bad: %#v", config)
	}

	if config.Ports.HTTP != 1234 {
		t.Fatalf("bad: %#v", config)
	}

	if config.Ports.RPC != 8100 {
		t.Fatalf("bad: %#v", config)
	}

	// Serf configs
	input = `{"ports": {"serf_lan": 1000, "serf_wan": 2000}}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.Ports.SerfLan != 1000 {
		t.Fatalf("bad: %#v", config)
	}

	if config.Ports.SerfWan != 2000 {
		t.Fatalf("bad: %#v", config)
	}

	// Server addrs
	input = `{"ports": {"server": 8000}, "bind_addr": "127.0.0.2", "advertise_addr": "127.0.0.3"}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.BindAddr != "127.0.0.2" {
		t.Fatalf("bad: %#v", config)
	}

	if config.AdvertiseAddr != "127.0.0.3" {
		t.Fatalf("bad: %#v", config)
	}

	if config.Ports.Server != 8000 {
		t.Fatalf("bad: %#v", config)
	}

	// leave_on_terminate
	input = `{"leave_on_terminate": true}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.LeaveOnTerm != true {
		t.Fatalf("bad: %#v", config)
	}

	// skip_leave_on_interrupt
	input = `{"skip_leave_on_interrupt": true}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.SkipLeaveOnInt != true {
		t.Fatalf("bad: %#v", config)
	}

	// enable_debug
	input = `{"enable_debug": true}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.EnableDebug != true {
		t.Fatalf("bad: %#v", config)
	}

	// TLS
	input = `{"verify_incoming": true, "verify_outgoing": true}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.VerifyIncoming != true {
		t.Fatalf("bad: %#v", config)
	}

	if config.VerifyOutgoing != true {
		t.Fatalf("bad: %#v", config)
	}

	// TLS keys
	input = `{"ca_file": "my/ca/file", "cert_file": "my.cert", "key_file": "key.pem", "server_name": "example.com"}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.CAFile != "my/ca/file" {
		t.Fatalf("bad: %#v", config)
	}
	if config.CertFile != "my.cert" {
		t.Fatalf("bad: %#v", config)
	}
	if config.KeyFile != "key.pem" {
		t.Fatalf("bad: %#v", config)
	}
	if config.ServerName != "example.com" {
		t.Fatalf("bad: %#v", config)
	}

	// Start join
	input = `{"start_join": ["1.1.1.1", "2.2.2.2"]}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if len(config.StartJoin) != 2 {
		t.Fatalf("bad: %#v", config)
	}
	if config.StartJoin[0] != "1.1.1.1" {
		t.Fatalf("bad: %#v", config)
	}
	if config.StartJoin[1] != "2.2.2.2" {
		t.Fatalf("bad: %#v", config)
	}

	// Retry join
	input = `{"retry_join": ["1.1.1.1", "2.2.2.2"]}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if len(config.RetryJoin) != 2 {
		t.Fatalf("bad: %#v", config)
	}
	if config.RetryJoin[0] != "1.1.1.1" {
		t.Fatalf("bad: %#v", config)
	}
	if config.RetryJoin[1] != "2.2.2.2" {
		t.Fatalf("bad: %#v", config)
	}

	// Retry interval
	input = `{"retry_interval": "10s"}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.RetryIntervalRaw != "10s" {
		t.Fatalf("bad: %#v", config)
	}
	if config.RetryInterval.String() != "10s" {
		t.Fatalf("bad: %#v", config)
	}

	// Retry Max
	input = `{"retry_max": 3}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.RetryMaxAttempts != 3 {
		t.Fatalf("bad: %#v", config)
	}

	// UI Dir
	input = `{"ui_dir": "/opt/consul-ui"}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.UiDir != "/opt/consul-ui" {
		t.Fatalf("bad: %#v", config)
	}

	// Pid File
	input = `{"pid_file": "/tmp/consul/pid"}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.PidFile != "/tmp/consul/pid" {
		t.Fatalf("bad: %#v", config)
	}

	// Syslog
	input = `{"enable_syslog": true, "syslog_facility": "LOCAL4"}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if !config.EnableSyslog {
		t.Fatalf("bad: %#v", config)
	}
	if config.SyslogFacility != "LOCAL4" {
		t.Fatalf("bad: %#v", config)
	}

	// Rejoin
	input = `{"rejoin_after_leave": true}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if !config.RejoinAfterLeave {
		t.Fatalf("bad: %#v", config)
	}

	// DNS node ttl, max stale
	input = `{"dns_config": {"node_ttl": "5s", "max_stale": "15s", "allow_stale": true}}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.DNSConfig.NodeTTL != 5*time.Second {
		t.Fatalf("bad: %#v", config)
	}
	if config.DNSConfig.MaxStale != 15*time.Second {
		t.Fatalf("bad: %#v", config)
	}
	if !config.DNSConfig.AllowStale {
		t.Fatalf("bad: %#v", config)
	}

	// DNS service ttl
	input = `{"dns_config": {"service_ttl": {"*": "1s", "api": "10s", "web": "30s"}}}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.DNSConfig.ServiceTTL["*"] != time.Second {
		t.Fatalf("bad: %#v", config)
	}
	if config.DNSConfig.ServiceTTL["api"] != 10*time.Second {
		t.Fatalf("bad: %#v", config)
	}
	if config.DNSConfig.ServiceTTL["web"] != 30*time.Second {
		t.Fatalf("bad: %#v", config)
	}

	// DNS enable truncate
	input = `{"dns_config": {"enable_truncate": true}}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if !config.DNSConfig.EnableTruncate {
		t.Fatalf("bad: %#v", config)
	}

	// CheckUpdateInterval
	input = `{"check_update_interval": "10m"}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.CheckUpdateInterval != 10*time.Minute {
		t.Fatalf("bad: %#v", config)
	}

	// ACLs
	input = `{"acl_token": "1234", "acl_datacenter": "dc2",
	"acl_ttl": "60s", "acl_down_policy": "deny",
	"acl_default_policy": "deny", "acl_master_token": "2345"}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.ACLToken != "1234" {
		t.Fatalf("bad: %#v", config)
	}
	if config.ACLMasterToken != "2345" {
		t.Fatalf("bad: %#v", config)
	}
	if config.ACLDatacenter != "dc2" {
		t.Fatalf("bad: %#v", config)
	}
	if config.ACLTTL != 60*time.Second {
		t.Fatalf("bad: %#v", config)
	}
	if config.ACLDownPolicy != "deny" {
		t.Fatalf("bad: %#v", config)
	}
	if config.ACLDefaultPolicy != "deny" {
		t.Fatalf("bad: %#v", config)
	}

	// Watches
	input = `{"watches": [{"type":"keyprefix", "prefix":"foo/", "handler":"foobar"}]}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if len(config.Watches) != 1 {
		t.Fatalf("bad: %#v", config)
	}

	out := config.Watches[0]
	exp := map[string]interface{}{
		"type":    "keyprefix",
		"prefix":  "foo/",
		"handler": "foobar",
	}
	if !reflect.DeepEqual(out, exp) {
		t.Fatalf("bad: %#v", config)
	}

	// remote exec
	input = `{"disable_remote_exec": true}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if !config.DisableRemoteExec {
		t.Fatalf("bad: %#v", config)
	}

	// stats(d|ite) exec
	input = `{"statsite_addr": "127.0.0.1:7250", "statsd_addr": "127.0.0.1:7251"}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.StatsiteAddr != "127.0.0.1:7250" {
		t.Fatalf("bad: %#v", config)
	}
	if config.StatsdAddr != "127.0.0.1:7251" {
		t.Fatalf("bad: %#v", config)
	}

	// Address overrides
	input = `{"addresses": {"dns": "0.0.0.0", "http": "127.0.0.1", "rpc": "127.0.0.1"}}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.Addresses.DNS != "0.0.0.0" {
		t.Fatalf("bad: %#v", config)
	}
	if config.Addresses.HTTP != "127.0.0.1" {
		t.Fatalf("bad: %#v", config)
	}
	if config.Addresses.RPC != "127.0.0.1" {
		t.Fatalf("bad: %#v", config)
	}

	// Disable updates
	input = `{"disable_update_check": true, "disable_anonymous_signature": true}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if !config.DisableUpdateCheck {
		t.Fatalf("bad: %#v", config)
	}
	if !config.DisableAnonymousSignature {
		t.Fatalf("bad: %#v", config)
	}
}

func TestDecodeConfig_Service(t *testing.T) {
	// Basics
	input := `{"service": {"id": "red1", "name": "redis", "tags": ["master"], "port":8000, "check": {"script": "/bin/check_redis", "interval": "10s", "ttl": "15s" }}}`
	config, err := DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if len(config.Services) != 1 {
		t.Fatalf("missing service")
	}

	serv := config.Services[0]
	if serv.ID != "red1" {
		t.Fatalf("bad: %v", serv)
	}

	if serv.Name != "redis" {
		t.Fatalf("bad: %v", serv)
	}

	if !strContains(serv.Tags, "master") {
		t.Fatalf("bad: %v", serv)
	}

	if serv.Port != 8000 {
		t.Fatalf("bad: %v", serv)
	}

	if serv.Check.Script != "/bin/check_redis" {
		t.Fatalf("bad: %v", serv)
	}

	if serv.Check.Interval != 10*time.Second {
		t.Fatalf("bad: %v", serv)
	}

	if serv.Check.TTL != 15*time.Second {
		t.Fatalf("bad: %v", serv)
	}
}

func TestDecodeConfig_Check(t *testing.T) {
	// Basics
	input := `{"check": {"id": "chk1", "name": "mem", "notes": "foobar", "script": "/bin/check_redis", "interval": "10s", "ttl": "15s" }}`
	config, err := DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if len(config.Checks) != 1 {
		t.Fatalf("missing check")
	}

	chk := config.Checks[0]
	if chk.ID != "chk1" {
		t.Fatalf("bad: %v", chk)
	}

	if chk.Name != "mem" {
		t.Fatalf("bad: %v", chk)
	}

	if chk.Notes != "foobar" {
		t.Fatalf("bad: %v", chk)
	}

	if chk.Script != "/bin/check_redis" {
		t.Fatalf("bad: %v", chk)
	}

	if chk.Interval != 10*time.Second {
		t.Fatalf("bad: %v", chk)
	}

	if chk.TTL != 15*time.Second {
		t.Fatalf("bad: %v", chk)
	}
}

func TestMergeConfig(t *testing.T) {
	a := &Config{
		Bootstrap:              false,
		BootstrapExpect:        0,
		Datacenter:             "dc1",
		DataDir:                "/tmp/foo",
		DNSRecursor:            "127.0.0.1:1001",
		Domain:                 "basic",
		LogLevel:               "debug",
		NodeName:               "foo",
		ClientAddr:             "127.0.0.1",
		BindAddr:               "127.0.0.1",
		AdvertiseAddr:          "127.0.0.1",
		Server:                 false,
		LeaveOnTerm:            false,
		SkipLeaveOnInt:         false,
		EnableDebug:            false,
		CheckUpdateIntervalRaw: "8m",
		RetryIntervalRaw:       "10s",
	}

	b := &Config{
		Bootstrap:       true,
		BootstrapExpect: 3,
		Datacenter:      "dc2",
		DataDir:         "/tmp/bar",
		DNSRecursor:     "127.0.0.2:1001",
		DNSConfig: DNSConfig{
			NodeTTL: 10 * time.Second,
			ServiceTTL: map[string]time.Duration{
				"api": 10 * time.Second,
			},
			AllowStale:     true,
			MaxStale:       30 * time.Second,
			EnableTruncate: true,
		},
		Domain:        "other",
		LogLevel:      "info",
		NodeName:      "baz",
		ClientAddr:    "127.0.0.1",
		BindAddr:      "127.0.0.1",
		AdvertiseAddr: "127.0.0.1",
		Ports: PortConfig{
			DNS:     1,
			HTTP:    2,
			RPC:     3,
			SerfLan: 4,
			SerfWan: 5,
			Server:  6,
		},
		Addresses: AddressConfig{
			DNS:  "127.0.0.1",
			HTTP: "127.0.0.2",
			RPC:  "127.0.0.3",
		},
		Server:                 true,
		LeaveOnTerm:            true,
		SkipLeaveOnInt:         true,
		EnableDebug:            true,
		VerifyIncoming:         true,
		VerifyOutgoing:         true,
		CAFile:                 "test/ca.pem",
		CertFile:               "test/cert.pem",
		KeyFile:                "test/key.pem",
		Checks:                 []*CheckDefinition{nil},
		Services:               []*ServiceDefinition{nil},
		StartJoin:              []string{"1.1.1.1"},
		UiDir:                  "/opt/consul-ui",
		EnableSyslog:           true,
		RejoinAfterLeave:       true,
		RetryJoin:              []string{"1.1.1.1"},
		RetryIntervalRaw:       "10s",
		RetryInterval:          10 * time.Second,
		CheckUpdateInterval:    8 * time.Minute,
		CheckUpdateIntervalRaw: "8m",
		ACLToken:               "1234",
		ACLMasterToken:         "2345",
		ACLDatacenter:          "dc2",
		ACLTTL:                 15 * time.Second,
		ACLTTLRaw:              "15s",
		ACLDownPolicy:          "deny",
		ACLDefaultPolicy:       "deny",
		Watches: []map[string]interface{}{
			map[string]interface{}{
				"type":    "keyprefix",
				"prefix":  "foo/",
				"handler": "foobar",
			},
		},
		DisableRemoteExec:         true,
		StatsiteAddr:              "127.0.0.1:7250",
		StatsdAddr:                "127.0.0.1:7251",
		DisableUpdateCheck:        true,
		DisableAnonymousSignature: true,
	}

	c := MergeConfig(a, b)

	if !reflect.DeepEqual(c, b) {
		t.Fatalf("should be equal %v %v", c, b)
	}
}

func TestReadConfigPaths_badPath(t *testing.T) {
	_, err := ReadConfigPaths([]string{"/i/shouldnt/exist/ever/rainbows"})
	if err == nil {
		t.Fatal("should have err")
	}
}

func TestReadConfigPaths_file(t *testing.T) {
	tf, err := ioutil.TempFile("", "consul")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	tf.Write([]byte(`{"node_name":"bar"}`))
	tf.Close()
	defer os.Remove(tf.Name())

	config, err := ReadConfigPaths([]string{tf.Name()})
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.NodeName != "bar" {
		t.Fatalf("bad: %#v", config)
	}
}

func TestReadConfigPaths_dir(t *testing.T) {
	td, err := ioutil.TempDir("", "consul")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.RemoveAll(td)

	err = ioutil.WriteFile(filepath.Join(td, "a.json"),
		[]byte(`{"node_name": "bar"}`), 0644)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	err = ioutil.WriteFile(filepath.Join(td, "b.json"),
		[]byte(`{"node_name": "baz"}`), 0644)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// A non-json file, shouldn't be read
	err = ioutil.WriteFile(filepath.Join(td, "c"),
		[]byte(`{"node_name": "bad"}`), 0644)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	config, err := ReadConfigPaths([]string{td})
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.NodeName != "baz" {
		t.Fatalf("bad: %#v", config)
	}
}
