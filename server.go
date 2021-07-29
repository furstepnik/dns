package main

import (
	"flag"
	"github.com/go-yaml/yaml"
	"io/ioutil"
	"log"
	"miekg/dns"
	"net"
	"os"
	"strings"
)

var (
	config    *Config
	configFile = flag.String("", "configuration.yaml", "configuration file")
)

type Key struct {
	name string
	rrType uint16
}

var rrMap map[Key]dns.RR = map[Key]dns.RR{}

type Config struct {
	udpAddress    string      `yaml:"udpAddress"`
	tcpAddress    string      `yaml:"tcpAddress"`
	udpSize       int         `yaml:"udpSize"`
}

func loadConfig() (*Config, error) {
	config := &Config{}

	if _, err := os.Stat(*configFile); err != nil {return nil, err}
	data, err := ioutil.ReadFile(*configFile)
	if err != nil {return nil, err}
	err = yaml.Unmarshal(data, config)
	if err != nil {return nil, err}

	return config, nil
}

func HandTCP(w dns.ResponseWriter, req *dns.Msg) {
	Handler(w, req)
}

func HandUDP(w dns.ResponseWriter, req *dns.Msg) {
	Handler(w, req)
}

func Handler(w dns.ResponseWriter, r *dns.Msg) {
	msg := dns.Msg{}
	msg.SetReply(r)
	z := Key{name: r.Question[0].Name, rrType: r.Question[0].Qtype}
	rrAns,ok := rrMap[z]
	msg.Authoritative = true
	if ok {
		if rrAns.Header().Rrtype==dns.TypeA {
			msg.Answer = append(msg.Answer, &dns.A{
				Hdr: dns.RR_Header{ Name: rrAns.Header().Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60 },
				A: net.ParseIP(rrAns.String()),
			})
		} else
		if rrAns.Header().Rrtype==dns.TypeAAAA {
			msg.Answer = append(msg.Answer, &dns.AAAA{
				Hdr:  dns.RR_Header{ Name: rrAns.Header().Name, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 60 },
				AAAA: net.IP(rrAns.String()),
			})
		} else
		if rrAns.Header().Rrtype==dns.TypeMX {
			msg.Answer = append(msg.Answer, &dns.MX{
				Hdr: dns.RR_Header{ Name: rrAns.Header().Name, Rrtype: dns.TypeMX, Class: dns.ClassINET, Ttl: 60 },
				Mx: rrAns.String(),
			})
		} else
		if rrAns.Header().Rrtype==dns.TypeNS {
			msg.Answer = append(msg.Answer, &dns.NS{
				Hdr: dns.RR_Header{ Name: rrAns.Header().Name, Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 60 },
				Ns: rrAns.String(),
			})
		}
		if rrAns.Header().Rrtype==dns.TypeTXT {
			msg.Answer = append(msg.Answer, &dns.TXT{
				Hdr: dns.RR_Header{ Name: rrAns.Header().Name, Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 60 },
				Txt: []string{rrAns.String()},
			})
		}
	}
	w.WriteMsg(&msg)
}

func readFile() {
	file, err := os.Open("example.com.txt")
	if err != nil {
		panic(err)
	}
	defer file.Close()
	content, err := ioutil.ReadAll(file)

	zp := dns.NewZoneParser(strings.NewReader(string(content)), "", "")
	for rr, ok := zp.Next(); ok; rr, ok = zp.Next() {
		z := Key{name: rr.Header().Name, rrType: rr.Header().Rrtype}
		rrMap[z] = rr
	}
}

func startServer() {
	tcpHandler := dns.NewServeMux()
	tcpHandler.HandleFunc(".", HandTCP)

	udpHandler := dns.NewServeMux()
	udpHandler.HandleFunc(".", HandUDP)

	tcpServer := &dns.Server{Addr: config.tcpAddress,
		Net:          "tcp",
		Handler:      tcpHandler,
	}

	udpServer := &dns.Server{Addr: config.udpAddress,
		Net:          "udp",
		Handler:      udpHandler,
		UDPSize:      config.udpSize,
	}

	readFile()
	go func() {
		if err := tcpServer.ListenAndServe(); err != nil {
			log.Fatal("TCP failed", err.Error())
		}
	}()
	go func() {
		if err := udpServer.ListenAndServe(); err != nil {
			log.Fatal("UDP failed", err.Error())
		}
	}()
}

func main() {
	config, _ = loadConfig()
	startServer()
}