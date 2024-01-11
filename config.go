package main

import (
	"github.com/ktcunreal/toriix/smux"
	"encoding/json"
	"flag"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Ingress string `json:"ingress"`
	Mode    string `json:"mode"`
	Egress  string `json:"egress"`
	PSK     string `json:"key"`
	keyring smux.Keyring
}

func readFromConfig() *Config {
	c := flag.String("c", "", "Configuration path")
	i := flag.String("i", "", "ingress listen address")
	e := flag.String("e", "", "egress address")
	p := flag.String("p", "", "pre shared key")
	m := flag.String("m", "", "mode")
	flag.Parse()

	conf := &Config{}

	if *c != "" {
		file, err := os.Open(*c)
		defer file.Close()
		log.Printf("Loading config from %s", *c)
		if err != nil {
			log.Fatalf("Loading config failed %v", err)
		}
		if err := json.NewDecoder(file).Decode(conf); err != nil {
			log.Fatalf("Parsing config failed: %v", err)
		}
		return conf
	}

	conf.Ingress = *i
	conf.Egress = *e
	conf.PSK = *p
	conf.Mode = *m

	if !ValidateConf(conf) {
		log.Fatalln("Config is not valid!")
	}

	return conf
}
func ValidateConf(c *Config) bool {
	// Check Addr length
	if len(c.Ingress)*len(c.Egress) == 0 {
		return false
	}

	// Check Ingress Ip:Port
	var ip, port string
	i := strings.LastIndexByte(c.Ingress, ':')
	if i == -1 {
		return false
	}
	ip, port = c.Ingress[:i], c.Ingress[i+1:]
	if len(ip) == 0 || net.ParseIP(ip) == nil {
		return false
	}
	if p, err := strconv.Atoi(port); err != nil || p > 65535 || p < 1 {
		return false
	}

	// Check Egress Ip:Port
	i = strings.LastIndexByte(c.Egress, ':')
	if i == -1 {
		return false
	}
	ip, port = c.Egress[:i], c.Egress[i+1:]
	if len(ip) == 0 || net.ParseIP(ip) == nil {

		return false
	}
	if p, err := strconv.Atoi(port); err != nil || p > 65535 || p < 1 {

		return false
	}

	return true
}
