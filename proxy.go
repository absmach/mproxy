package mgate

import (
	"io"
	"net"
)

type Proxy interface {
	HandleConnection(conn net.Conn)
	Pipe(src io.Reader, dst io.Writer, h HandleFunc, conn ConnectionInfo)
}
