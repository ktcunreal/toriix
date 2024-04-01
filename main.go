package main

import (
	"github.com/djherbis/buffer"
	"github.com/djherbis/nio/v3"
	"github.com/ktcunreal/toriix/smux"
	"io"
	"log"
	"net"
	"sync"
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
		conf      *Config
		wg        sync.WaitGroup
		clientCnt uint8
	}{
		conf:      c,
		wg:        sync.WaitGroup{},
		clientCnt: 0,
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
				server.clientCnt += 1
				log.Printf("Client num is: %v\n", server.clientCnt)

				for {
					// Accept smux stream
					src, err := session.AcceptStream()
					if err != nil {
						server.clientCnt -= 1
						log.Printf("%v\nClient num is: %v\n", err, server.clientCnt)
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
					go NPipe(src, dst)
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
			for {
				log.Println("creating new session")
				conn, err := net.Dial("tcp", client.conf.Egress)
				if err != nil {
					log.Println("Tcp server unreachable")
					time.Sleep(time.Second)
					continue
				}
				defer conn.Close()

				session, err := smux.Client(conn, nil, client.conf.PSK)
				if err != nil {
					log.Printf("Smux session down: %v\n", err)
					continue
				}
				defer session.Close()

				for i := 0; i < 8; i++ {
					src, err := listener.Accept()
					if err != nil {
						log.Printf("Failed to accept incoming tcp connection: %v\n", err)
						continue
					}
					//defer src.Close()

					if session.IsClosed() {
						log.Printf("Session unexpectly closed...\n")
					}
					str, err := session.OpenStream()
					if err != nil {
						log.Printf("Smux stream down: %v\n", err)
						break
					}
					//defer str.Close()

					go NPipe(src, str)
				}
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

func NPipe2(src, dst io.ReadWriteCloser) {
	go func() {
		defer src.Close()
		if _, err := nio.Copy(dst, src, buffer.New(32*1024)); err != nil {
			if err == io.EOF {
				log.Println("EOF")
			}
			log.Println(err)
			return
		}
	}()
	go func() {
		defer dst.Close()
		if _, err := nio.Copy(src, dst, buffer.New(32*1024)); err != nil {
			if err == io.EOF {
				log.Println("EOF")
			}
			log.Println(err)
			return
		}
	}()
}

func NPipe(src, dst io.ReadWriteCloser) {
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		if _, err := nio.Copy(dst, src, buffer.New(32*1024)); err != nil {
			log.Println(err)
			return
		}
	}()
	go func() {
		defer wg.Done()
		if _, err := nio.Copy(src, dst, buffer.New(32*1024)); err != nil {
			log.Println(err)
			return
		}
	}()

	wg.Wait()
	src.Close()
	dst.Close()
}
