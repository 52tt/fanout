package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

type client struct {
	id int
    conn net.Conn
    ch chan []byte
}

var clients []*client

func main() {
	_help := flag.Bool("h", false, "Show help")
	_listenAddr := flag.String("l", "127.0.0.1:2000", "Listen address")
	flag.Parse()
	help := *_help
	if help {
		flag.PrintDefaults()
		os.Exit(1)
	}

	sigCh := make(chan os.Signal, 4)
	defer close(sigCh)
	connectedCh := make(chan net.Conn, 16)
	defer close(connectedCh)
	disconnectedCh := make(chan int, 16)
	defer close(disconnectedCh)
	stdinRxCh := make(chan []byte, 128)
	defer close(stdinRxCh)
	stdinClosedCh := make(chan struct{})
	defer close(stdinClosedCh)

	listenAddr := *_listenAddr
	go listener(listenAddr, connectedCh)
	go stdinReader(stdinRxCh, stdinClosedCh)

	signal.Notify(sigCh, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	globalClientId := 0
	var wg sync.WaitGroup
	for {
		select {
		case conn := <- connectedCh:
			var clientName string
			if conn.RemoteAddr().Network() == "unix" {
				// For unix domain socket, `conn.RemoteAddr().String()` always return "@"
				clientName = fmt.Sprintf("unix:%p", conn)
			} else {
				clientName = conn.RemoteAddr().String()
			}
			log.Printf("Client [%d] %s connected\n", globalClientId, clientName)
			c := client{
				id: globalClientId,
				conn: conn,
				ch: make(chan []byte, 64),
			}
			clients = append(clients, &c)
			globalClientId++
			wg.Add(1)
			go func() {
				serve(&c)
				disconnectedCh <- c.id
				wg.Done()
			}()

		case cid := <- disconnectedCh:
			// remove client
			for i, c := range clients {
				if c.id == cid {
					close(c.ch)
					clients = append(clients[:i], clients[i+1:]...)
					log.Printf("Client [%d] removed\n", c.id)
					break
				}
			}

		case <-sigCh:
			signal.Stop(sigCh)
			goto quit

		case <-stdinClosedCh:
			goto quit

		case data := <-stdinRxCh:
			// Write to stdout
			os.Stdout.Write(data)

			// Write to clients channel
			for _, c := range clients {
				select {
				case c.ch <- data:
					;
				default:
					log.Printf("Client [%d] is slow\n", c.id)
				}
			}
		}
	}

quit:
	// cleanup
	_, _, err := net.SplitHostPort(listenAddr)
	if err != nil { // unix domain socket
		os.Remove(listenAddr)
	}
	for _, c := range clients {
		close(c.ch)
	}
	wg.Wait()
}

func listener(listenAddr string, connectedCh chan net.Conn) {
	sockType := "tcp"
	_, _, err := net.SplitHostPort(listenAddr)
	if err != nil {
		sockType = "unix"
	}
	l, err := net.Listen(sockType, listenAddr)
	if err != nil {
		log.Fatalf("Listen on %s error: %s\n", listenAddr, err)
	}
	defer l.Close()

	log.Println("Listening on", listenAddr)
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Println("Accept error:", err)
		} else {
			connectedCh <- conn
		}
	}
}

func serve(c *client) {
	for {
		data, ok := <-c.ch
		if !ok {
			goto quit
		}

		offset := 0
		for {
			n, err := c.conn.Write(data[offset:])
			if err != nil {
				log.Printf("Write to client [%d] at offset %d failed: %s\n", c.id, offset, err)
				goto quit
			} else if (n > 0) {
				offset += n
				if offset >= len(data) {
					break
				} else {
					log.Printf("Client [%d]: sent %d/%d\n", c.id, offset, len(data))
				}
			}
		}
	}

quit:
	err := c.conn.Close()
	if err != nil {
		log.Printf("Close client [%d] failed: %s\n", c.id, err)
	}
}

func stdinReader(ch chan []byte, stdinClosedCh chan struct{}) {
	reader := bufio.NewReader(os.Stdin)
	for {
		input, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				log.Println("Read EOF from stdin");
			} else {
				log.Println("Read from stdin failed:", err);
			}
			stdinClosedCh <- struct{}{}
			break
		}
		ch <- input
	}
}
