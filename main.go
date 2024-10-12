package main

import (
	"github.com/ktcunreal/toriix/smux"
	"io"
	"log"
	"net"
	"time"
)

var (
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
	}{
		conf: c,
	}

	listener := initListener(server.conf.Ingress)
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept incoming tcp connection %v", err)
			continue
		}
		defer conn.Close()

		go func() {
			session, err := smux.Server(conn, nil, server.conf.PSK)
			if err != nil {
				log.Printf("Failed to create smux session: %v\n", err)
				return
			}
			defer session.Close()

			for {
				// Accept smux stream
				src, err := session.AcceptStream()
				if err != nil {
					log.Printf("Failed to accept smux stream: %v\n", err)
					if err == smux.ErrInvalidProtocol || err == smux.ErrDecryptFailed {
						io.Copy(io.Discard, conn)
					}
					session.Close()
					conn.Close()
					log.Printf("Remote peer disconnected")
					return
				}
				defer src.Close()

				// Establish Remote TCP connection
				var dst net.Conn
				for i := 0; i < 3; i++ {
					dst, err = net.Dial("tcp", server.conf.Egress)
					if err != nil {
						log.Printf("Upstream service unreachable: %v", err)
						time.Sleep(time.Second)
						continue
					}
					break
				}

				if dst == nil {
					src.Close()
					continue
				}
				defer dst.Close()

				// forwarding
				go smux.KPipe(src, dst, 10)
			}
		}()
	}
}

func client(c *Config) {
	client := struct {
		conf *Config
	}{
		conf: c,
	}

	listener := initListener(client.conf.Ingress)
	defer listener.Close()
	for {
	NewConn:
		for {
			conn, err := net.Dial("tcp", client.conf.Egress)
			if err != nil {
				log.Println("Tcp server unreachable")
				time.Sleep(time.Second * 3)
				continue
			}
			defer conn.Close()

			session, err := smux.Client(conn, nil, client.conf.PSK)
			if err != nil {
				log.Printf("Failed to create smux session: %v\n", err)
				continue
			}
			defer session.Close()

			for {
				src, err := listener.Accept()
				if err != nil {
					log.Printf("Failed to accept incoming tcp connection: %v\n", err)
					continue
				}
				defer src.Close()

				stream, err := session.OpenStream()
				if err != nil {
					log.Printf("Smux stream down: %v\n", err)
					src.Close()
					session.Close()
					conn.Close()
					break NewConn
				}
				defer stream.Close()

				go smux.KPipe(src, stream, 10)
			}
		}
	}
}

func initListener(addr string) net.Listener {
	defer log.Printf("LISTENER STARTED ON %s", addr)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalln("LISTENER FAILED TO START: ", err)
	}
	return listener
}
