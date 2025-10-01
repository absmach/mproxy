package mgate

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

// ConnectionInfo holds metadata about the connection
type ConnectionInfo struct {
	ClientAddr string
	ServerAddr string
	StartTime  time.Time
}

// TCPProxy represents the proxy server
type TCPProxy struct {
	listenAddr string
	targetAddr string
	handlers   []ProtocolHandler
	listener   net.Listener
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

func NewTCPProxy(listenAddr, targetAddr string) *TCPProxy {
	ctx, cancel := context.WithCancel(context.Background())
	return &TCPProxy{
		listenAddr: listenAddr,
		targetAddr: targetAddr,
		handlers:   make([]ProtocolHandler, 0),
		ctx:        ctx,
		cancel:     cancel,
	}
}

func (p *TCPProxy) AddHandler(handler ProtocolHandler) {
	p.handlers = append(p.handlers, handler)
}

func (p *TCPProxy) Start() error {
	var err error
	p.listener, err = net.Listen("tcp", p.listenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	log.Printf("TCP Proxy listening on %s, forwarding to %s", p.listenAddr, p.targetAddr)

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		p.acceptLoop()
	}()

	return nil
}

func (p *TCPProxy) acceptLoop() {
	for {
		clientConn, err := p.listener.Accept()
		if err != nil {
			select {
			case <-p.ctx.Done():
				return
			default:
				log.Printf("Accept error: %v", err)
				continue
			}
		}

		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			p.handleConnection(clientConn)
		}()
	}
}

func (p *TCPProxy) handleConnection(clientConn net.Conn) {
	defer clientConn.Close()

	serverConn, err := net.Dial("tcp", p.targetAddr)
	if err != nil {
		log.Printf("Failed to connect to target: %v", err)
		return
	}
	defer serverConn.Close()

	connInfo := ConnectionInfo{
		ClientAddr: clientConn.RemoteAddr().String(),
		ServerAddr: serverConn.RemoteAddr().String(),
		StartTime:  time.Now(),
	}

	// Peek at initial data to detect protocol
	clientConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	reader := bufio.NewReader(clientConn)
	initialData, err := reader.Peek(128)
	clientConn.SetReadDeadline(time.Time{})

	var handler ProtocolHandler
	if err == nil {
		for _, h := range p.handlers {
			if h.Detect(initialData) {
				handler = h
				break
			}
		}
	}

	if handler == nil {
		handler = NewDefaultHandler()
	}

	if err := handler.OnConnect(p.ctx, connInfo); err != nil {
		log.Printf("OnConnect error: %v", err)
		return
	}
	defer handler.OnClose(p.ctx, connInfo)

	// Create pipes for bidirectional communication
	var wg sync.WaitGroup
	wg.Add(2)

	// Client to Server
	go func() {
		defer wg.Done()
		p.pipe(reader, serverConn, handler.HandleClientData, connInfo)
	}()

	// Server to Client
	go func() {
		defer wg.Done()
		p.pipe(serverConn, clientConn, handler.HandleServerData, connInfo)
	}()

	wg.Wait()
}

func (p *TCPProxy) pipe(src io.Reader, dst io.Writer, handleFunc func(context.Context, []byte, ConnectionInfo) ([]byte, bool, error), conn ConnectionInfo) {
	buf := make([]byte, 32*1024)
	for {
		select {
		case <-p.ctx.Done():
			return
		default:
		}

		n, err := src.Read(buf)
		if err != nil {
			if err != io.EOF {
				log.Printf("Read error: %v", err)
			}
			return
		}

		data, cont, err := handleFunc(p.ctx, buf[:n], conn)
		if err != nil {
			log.Printf("Handler error: %v", err)
			return
		}

		if !cont {
			return
		}

		if _, err := dst.Write(data); err != nil {
			log.Printf("Write error: %v", err)
			return
		}
	}
}

func (p *TCPProxy) Stop() error {
	p.cancel()
	if p.listener != nil {
		p.listener.Close()
	}
	p.wg.Wait()
	return nil
}
