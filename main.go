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
	}{
		conf: c,
		wg:   sync.WaitGroup{},
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

			session, err := smux.Server(conn, nil, server.conf.PSK)
			if err != nil {
				log.Printf("%v\n", err)
				continue
			}

			go func() {
				for {
					var src *smux.Stream
					for i := 0; i < 3; i++ {
						src, err = session.AcceptStream()
						if err != nil {
							if session.IsClosed() {
								log.Printf("Session closed.\n")
								return
							}
							continue
						}
						break
					}

					if src == nil {
						defer session.Close()
						defer conn.Close()
						log.Printf("Unable to accept stream.\n")
						return
					}

					// Establish Remote TCP connection
					var dst net.Conn
					for i := 0; i < 5; i++ {
						dst, err = net.Dial("tcp", server.conf.Egress)
						if err != nil {
							log.Printf("Upstream service unreachable: %v", err)
							time.Sleep(time.Millisecond * 300)
							continue
						}
						break
					}

					if dst == nil {
						log.Printf("Unable to reach upstream service")
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
			src, err := listener.Accept()
			if err != nil {
				log.Printf("Failed to accept incoming tcp connection: %v\n", err)
				continue
			}
			defer src.Close()

			var conn net.Conn
			var session *smux.Session
			var str *smux.Stream

			for {
				for conn == nil || session == nil {
					conn, err = net.Dial("tcp", client.conf.Egress)
					if err != nil {
						log.Println("Tcp server unreachable")
						time.Sleep(time.Second)
						continue
					}
					session, err = smux.Client(conn, nil, client.conf.PSK)
					if err != nil {
						log.Printf("Create smux session failed: %v\n", err)
						continue
					}
				}

				if session.IsClosed() {
					conn, session = nil, nil
					continue
				}

				for i := 0; i < 3; i++ {
					str, err = session.OpenStream()
					if err != nil {
						log.Printf("Unable to open smux stream: %v\n", err)
						time.Sleep(time.Millisecond * 300)
						continue
					}
					break
				}

				if str != nil {
					break
				}

				session.Close()
				log.Printf("Closing stalled session\n")
				time.Sleep(time.Millisecond * 300)
			}
			go NPipe(src, str)
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
