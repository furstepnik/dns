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

type zone struct {
	name string `yaml:"name"`
	path string `yaml:"path"`
}

var rrMap map[Key]dns.RR = map[Key]dns.RR{}
var fileNum = -1

type Config struct {
	addresses     []string    `yaml:"addresses"`
	zones         []zone    `yaml:"zones"`
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
	openFile := readFile(r.Question[0].Name)
	z := Key{name: r.Question[0].Name, rrType: r.Question[0].Qtype}
	if openFile == 1 {
		rrAns, ok := rrMap[z]
		if ok {
			msg := dns.Msg{}
			msg.SetReply(r)
			msg.Authoritative = true
			if rrAns.Header().Rrtype == dns.TypeA {
				msg.Answer = append(msg.Answer, &dns.A{
					Hdr: dns.RR_Header{Name: rrAns.Header().Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
					A:   net.ParseIP(rrAns.String()),
				})
			} else
			if rrAns.Header().Rrtype == dns.TypeAAAA {
				msg.Answer = append(msg.Answer, &dns.AAAA{
					Hdr:  dns.RR_Header{Name: rrAns.Header().Name, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 60},
					AAAA: net.IP(rrAns.String()),
				})
			} else
			if rrAns.Header().Rrtype == dns.TypeMX {
				msg.Answer = append(msg.Answer, &dns.MX{
					Hdr: dns.RR_Header{Name: rrAns.Header().Name, Rrtype: dns.TypeMX, Class: dns.ClassINET, Ttl: 60},
					Mx:  rrAns.String(),
				})
			} else
			if rrAns.Header().Rrtype == dns.TypeNS {
				msg.Answer = append(msg.Answer, &dns.NS{
					Hdr: dns.RR_Header{Name: rrAns.Header().Name, Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 60},
					Ns:  rrAns.String(),
				})
			}
			if rrAns.Header().Rrtype == dns.TypeTXT {
				msg.Answer = append(msg.Answer, &dns.TXT{
					Hdr: dns.RR_Header{Name: rrAns.Header().Name, Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 60},
					Txt: []string{rrAns.String()},
				})
			}
			w.WriteMsg(&msg)
		} else {
			msg := dns.Msg{}
			msg.SetRcode(r, dns.RcodeNameError)
			log.Println("DNS PROBE FINISHED NXDOMAIN")
		}
	} else {
		msg := dns.Msg{}
		msg.SetRcode(r, dns.RcodeNameError)
		log.Println("DNS PROBE FINISHED NXDOMAIN")
	}
}

func readFile(name string) (open int){

	zonePath := ""
	f := -1
	for i, z := range config.zones {
		s := z.name
		if len(name)>len(s) {
			s1 := name[len(name)-len(s):]
			if s1==s {
				zonePath = z.path
				f = i
			}
		}
	}

	if (f != -1) && (f != fileNum) {
		file, err := os.Open(zonePath)
		if err != nil {
			panic(err)
		}
		defer file.Close()
		content, err := ioutil.ReadAll(file)

		zp := dns.NewZoneParser(strings.NewReader(string(content)), "", "")
		rrMap := map[Key]dns.RR{}
		for rr, ok := zp.Next(); ok; rr, ok = zp.Next() {
			z := Key{name: rr.Header().Name, rrType: rr.Header().Rrtype}
			rrMap[z] = rr
		}
		fileNum = f
		open = 1
	} else
	if f == -1 {
		open = 0
	}
	return
}

func startServer() {
	tcpHandler := dns.NewServeMux()
	tcpHandler.HandleFunc(".", HandTCP)

	udpHandler := dns.NewServeMux()
	udpHandler.HandleFunc(".", HandUDP)

    for _, a := range config.addresses {
		tcpServer := &dns.Server{Addr: a,
			Net:     "tcp",
			Handler: tcpHandler,
		}

		udpServer := &dns.Server{Addr: a,
			Net:     "udp",
			Handler: udpHandler,
			UDPSize: 65535,
		}

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
}

func main() {
	config, _ = loadConfig()
	startServer()
}