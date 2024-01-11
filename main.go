package main

import (
	"./smux"
	"fmt"
	"io"
	"log"
	"net"
	"time"
	"sync"
)

func main() {
	f := readFromConfig()
	switch f.Mode {
	case "server":
		server(f)
	case "client":
		client(f)
	default:
		fmt.Println("please specify running mode!")		
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
				log.Printf("FAILED TO ACCEPT TCP CONNECTION: %v", err)
				continue
			}

			session, err := smux.Server(conn, nil, server.conf.PSK)
			if err != nil {
				fmt.Println(err)
			}
			defer session.Close()

			for {
				// Accept smux stream
				src, err := session.AcceptStream()
				if err != nil {
					fmt.Println(err)
					break
				}

				// Establish Remote TCP connection
				dst, err := net.Dial("tcp", server.conf.Egress)
				if err != nil {
					log.Printf("UPSTREAM SERVICE UNREACHABLE: %v", err)
					continue
				}
				// forwarding
				//csrc := NewS2Stream(src)
				Pipe(src, dst)
			}
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
				log.Println("TCP SERVER UNREACHABLE")
				time.Sleep(time.Second * 5)
				continue
			}
			session, err := smux.Client(conn, nil, client.conf.PSK)
			if err != nil {
				continue
			}
			defer session.Close()

			for {
				src, err := listener.Accept()
				if err != nil {
					log.Printf("FAILED TO ACCEPT TCP CONNECTION: %v", err)
					continue
				}

				str, err := session.OpenStream()
				if err != nil {
					log.Println("stream down")
					break
				}
				//cstr := NewS2Stream(str)
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
