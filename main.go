package main

import (
	"github.com/djherbis/buffer"
	"github.com/djherbis/nio/v3"
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
				log.Println(err)
				return
			}
			defer session.Close()

			for {
				// Accept smux stream
				src, err := session.AcceptStream()
				if err != nil {
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
				NPipe(src, dst)
			}
		}()
	}
}

func client(c *Config) {
	client := struct {
		conf       *Config
		rollerSize int
	}{
		conf:       c,
		rollerSize: 2 << 16,
	}

	listener := initListener(client.conf.Ingress)
	defer listener.Close()

	lst := make([]*smux.Session, client.rollerSize)

	for {
		for i := 0; i < client.rollerSize; i++ {
			conn, err := net.Dial("tcp", client.conf.Egress)
			if err != nil {
				log.Println("Tcp server unreachable")
				time.Sleep(time.Second)
				continue
			}
			defer conn.Close()

			lst[i], err = smux.Client(conn, nil, client.conf.PSK)
			if err != nil {
				log.Printf("Smux session down: %v\n", err)
				continue
			}
			defer lst[i].Close()

			if i > 0 && lst [i-1] != nil {
				go lst[i-1].Recycler()
			}

			for {
				if lst[i] == nil || lst[i].IsClosed() {
					break
				}

				src, err := listener.Accept()
				if err != nil {
					log.Printf("Failed to accept incoming tcp connection: %v\n", err)
					continue
				}
				defer src.Close()

				str, err := lst[i].OpenStream()
				if err != nil {
					log.Printf("Smux stream down: %v\n", err)
					src.Close()
					break
				}
				defer str.Close()
				NPipe(src, str)
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

func Pipe(src, dst io.ReadWriteCloser) {
	go func() {
		defer src.Close()
		defer dst.Close()
		if _, err := io.Copy(src, dst); err != nil {
			log.Println(err)
			return
		}
	}()
	go func() {
		defer src.Close()
		defer dst.Close()
		if _, err := io.Copy(dst, src); err != nil {
			log.Println(err)
			return
		}
	}()
}

func NPipe(src, dst io.ReadWriteCloser) {
	go func() {
		defer src.Close()
		if _, err := nio.Copy(dst, src, buffer.New(32*1024)); err != nil {
			//log.Println(err)
			return
		}
	}()
	go func() {
		defer dst.Close()
		if _, err := nio.Copy(src, dst, buffer.New(32*1024)); err != nil {
			//log.Println(err)
			return
		}
	}()
}
