package main

import (
	"encoding/json"
	"flag"
	"github.com/ktcunreal/toriix/smux"
	"log"
	"os"
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
	return true
}
