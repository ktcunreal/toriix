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
		conf *Config
		wg   sync.WaitGroup
		sessionCount uint8
	}{
		conf: c,
		wg:   sync.WaitGroup{},
		sessionCount: 0,
	}

	listener := initListener(server.conf.Ingress)
	defer listener.Close()
	server.wg.Add(1)
	go func() {
		defer server.wg.Done()
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
					log.Printf("%v\n", err)
					return
				}
				server.sessionCount += 1
				log.Printf("Session count: %v\n", server.sessionCount)

				for {
					var dst net.Conn
					// Accept smux stream
					src, err := session.AcceptStream()
					if err != nil {
						log.Printf("%v\n", err)
						server.sessionCount -= 1
						log.Printf("Session count: %v\n", server.sessionCount)
						return
					}

					// Establish Remote TCP connection
					for i := 0; i <= 5; i++ {
						dst, err = net.Dial("tcp", server.conf.Egress)
						if err != nil {
							log.Printf("Upstream service unreachable: %v", err)
							time.Sleep(time.Second)
							continue
						}
						break
					}

					if dst == nil {
						continue
					}
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
			var conn net.Conn
			var session *smux.Session

			for i := 0; i < 16; i++ {
				var str *smux.Stream

				src, err := listener.Accept()
				if err != nil {
					log.Printf("Failed to accept incoming tcp connection: %v\n", err)
					continue
				}
				defer src.Close()

				for {
					for conn == nil || session == nil || session.IsClosed() {
						log.Println("Creating new session")
						conn, err = net.Dial("tcp", client.conf.Egress)
						if err != nil {
							log.Println("Tcp server unreachable")
							time.Sleep(time.Second)
							continue
						}

						session, err = smux.Client(conn, nil, client.conf.PSK)
						if err != nil {
							log.Printf("Smux session down: %v\n", err)
							continue
						}
					}
					str, err = session.OpenStream()
					if err != nil {
						log.Printf("Smux stream unavailable: %v\n", err)
						time.Sleep(time.Second)
						continue
					}
					break
				}
				//defer conn.Close()
				//defer str.Close()
				//defer session.Close()
				go NPipe(src, str)
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
	defer src.Close()
	defer dst.Close()
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
}
