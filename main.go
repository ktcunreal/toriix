package main

import (
	"github.com/ktcunreal/toriix/smux"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

var (
	clientCnt = 0
	Version string
)

func main() {
	if len(Version) > 0 {
		log.Printf("Toriix version: %s\n", Version)
	} else {
		log.Printf("Running in dev mode")
	}

	f := readFromConfig()
	switch f.Mode {
	case "server":
		server(f)
	case "client":
		client(f)
	default:
		log.Fatalln("Please specify running mode!")
	}
}

func server(c *Config) {
	server := struct {
		conf *Config
		wg   sync.WaitGroup
	}{
		conf: c,
		wg:   sync.WaitGroup{},
	}

	listener := initListener(server.conf.Ingress)
	defer listener.Close()
	server.wg.Add(1)
	go func() {
		for {
			defer server.wg.Done()
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("Failed to accept incoming tcp connection %v", err)
				continue
			}
			defer conn.Close()

			go func() {
				session, err := smux.Server(conn, nil, server.conf.PSK)
				if err != nil {
					log.Println(err)
					return
				}
				clientCnt += 1
				log.Printf("Client num is: %v\n", clientCnt)
				defer session.Close()
				for {
					// Accept smux stream
					src, err := session.AcceptStream()
					if err != nil {
						clientCnt -= 1
						log.Printf("%v\nClient num is: %v\n", err, clientCnt)
						return
					}
					defer src.Close()

					// Establish Remote TCP connection
					dst, err := net.Dial("tcp", server.conf.Egress)
					if err != nil {
						log.Printf("Upstream service unreachable: %v", err)
						continue
					}
					defer dst.Close()
					// forwarding
					Pipe(src, dst)
				}
			}()
		}
	}()
	server.wg.Wait()
}

func client(c *Config) {
	client := struct {
		conf *Config
		wg   sync.WaitGroup
	}{
		conf: c,
		wg:   sync.WaitGroup{},
	}

	listener := initListener(client.conf.Ingress)
	defer listener.Close()
	client.wg.Add(1)
	go func() {
		defer client.wg.Done()
		for {
			conn, err := net.Dial("tcp", client.conf.Egress)
			if err != nil {
				log.Println("Tcp server unreachable")
				time.Sleep(time.Second * 5)
				continue
			}
			defer conn.Close()

			session, err := smux.Client(conn, nil, client.conf.PSK)
			if err != nil {
				continue
			}
			defer session.Close()

			for {
				src, err := listener.Accept()
				if err != nil {
					log.Printf("Failed to accept incoming tcp connection: %v", err)
					break
				}
				defer src.Close()

				str, err := session.OpenStream()
				if err != nil {
					log.Println("Smux stream down")
					break
				}
				defer str.Close()

				Pipe(src, str)
			}
		}
	}()

	client.wg.Wait()
}

func initListener(addr string) net.Listener {
	defer log.Printf("LISTENER STARTED ON %s", addr)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalln("LISTENER FAILED TO START: ", err)
	}
	return listener
}

func Pipe(src, dst io.ReadWriteCloser) {
	go func() {
		defer src.Close()
		defer dst.Close()
		if _, err := io.Copy(src, dst); err != nil {
			return
		}
	}()
	go func() {
		defer src.Close()
		defer dst.Close()
		if _, err := io.Copy(dst, src); err != nil {
			return
		}
	}()
}
