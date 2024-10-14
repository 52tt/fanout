package main

import (
	"flag"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
)

type client struct {
    name string
    ch chan []byte
}

var clients []client

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
	connectedCh := make(chan client, 1)
	defer close(connectedCh)
	disconnectedCh := make(chan string, 1)
	defer close(disconnectedCh)
	stdinRxCh := make(chan []byte, 128)
	defer close(stdinRxCh)
	stdinClosedCh := make(chan struct{})
	defer close(stdinClosedCh)

	listenAddr := *_listenAddr
	go listener(listenAddr, connectedCh, disconnectedCh)
	go stdinReader(stdinRxCh, stdinClosedCh)

	signal.Notify(sigCh, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
loop:
	for {
		select {
		case client := <- connectedCh:
			clients = append(clients, client)
			// log.Println("Added client", client.name)

		case clientName := <- disconnectedCh:
			// remove client
			for i, client := range clients {
				if client.name == clientName {
					close(client.ch)
					clients = append(clients[:i], clients[i+1:]...)
					// log.Println("Removed client", client.name)
					break
				}
			}

		case <-sigCh:
			signal.Stop(sigCh)
			break loop

		case <-stdinClosedCh:
			break loop

		case b := <-stdinRxCh:
			// Write to stdout
			os.Stdout.Write(b)

			// Write to clients
			for _, client := range clients {
				client.ch <- b
			}
		}
	}

	// cleanup
	_, _, err := net.SplitHostPort(listenAddr)
	if err != nil { // unix domain socket
		os.Remove(listenAddr)
	}
}

func listener(listenAddr string, connectedCh chan client, disconnectedCh chan string) {
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

	// log.Println("Listening on", listenAddr)
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Println("Accept error:", err)
		} else {
			go serve(conn, connectedCh, disconnectedCh)
		}
	}
}

func serve(conn net.Conn, connectedCh chan client, disconnectedCh chan string) {
	defer conn.Close()

	clientName := conn.RemoteAddr().String()
	ch := make(chan []byte, 64) // This will be closed by the main goroutine
	client := client{name: clientName, ch: ch}
	connectedCh <- client

	// log.Prinln("Serving client", clientName)
	for {
		b := <-ch
		_, err := conn.Write(b)
		if err != nil {
			break
		}
	}

	disconnectedCh <- clientName
}

func stdinReader(ch chan []byte, stdinClosedCh chan struct{}) {
	var buffer [64*1024]byte
	for {
		rxLen, err := os.Stdin.Read(buffer[:])
		if rxLen > 0 {
			ch <- buffer[:rxLen]
		} else if err != nil {
			if err != io.EOF {
				log.Println(err);
			}
			stdinClosedCh <- struct{}{}
			break
		}
	}
}
