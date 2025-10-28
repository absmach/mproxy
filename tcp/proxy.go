package tcp

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/absmach/mgate"
)

// TCPProxy represents the proxy server
type TCPProxy struct {
	listenAddr string
	targetAddr string
	buffSize   int
	handler    mgate.Handler
	listener   net.Listener
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

func New(listenAddr, targetAddr string, buffSize int, h mgate.Handler) *TCPProxy {
	ctx, cancel := context.WithCancel(context.Background())
	return &TCPProxy{
		listenAddr: listenAddr,
		targetAddr: targetAddr,
		handler:    h,
		buffSize:   buffSize,
		ctx:        ctx,
		cancel:     cancel,
	}
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

	connInfo := mgate.ConnectionInfo{
		Client: mgate.ClientSession{
			Addr: clientConn.RemoteAddr().String(),
		},
		Server: mgate.ServerSession{
			Addr: serverConn.RemoteAddr().String(),
		},
		StartTime: time.Now(),
	}

	if err := p.handler.OnConnect(p.ctx, connInfo); err != nil {
		log.Printf("OnConnect error: %v", err)
		return
	}
	defer p.handler.OnDisconnect(p.ctx, connInfo)

	// Create pipes for bidirectional communication
	var wg sync.WaitGroup
	wg.Add(2)

	// Client to Server
	go func() {
		defer wg.Done()
		p.pipe(clientConn, serverConn, p.handler.HandleClientData, connInfo)
	}()

	// Server to Client
	go func() {
		defer wg.Done()
		p.pipe(serverConn, clientConn, p.handler.HandleServerData, connInfo)
	}()

	wg.Wait()
}

func (p *TCPProxy) pipe(src io.Reader, dst io.Writer, h handleFunc, conn mgate.ConnectionInfo) {
	buf := make([]byte, p.buffSize)
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

		data, cont, err := h(p.ctx, buf[:n], conn)
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

type handleFunc func(context.Context, []byte, mgate.ConnectionInfo) ([]byte, bool, error)
