package tunnel

import (
	"net"
	"sync"
	"time"
)

const (
	tcpDialTimeout     = 10 * time.Second
	tcpKeepAlivePeriod = 30 * time.Second
)

var copyBufferPool = sync.Pool{
	New: func() any {
		buf := make([]byte, copyBufferSize)
		return &buf
	},
}

func dialNetwork(network, address string) (net.Conn, error) {
	dialer := net.Dialer{
		Timeout:   tcpDialTimeout,
		KeepAlive: tcpKeepAlivePeriod,
	}
	conn, err := dialer.Dial(network, address)
	if err != nil {
		return nil, err
	}
	configureTCPConn(conn)
	return conn, nil
}

func configureTCPConn(conn net.Conn) {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return
	}
	_ = tcpConn.SetNoDelay(true)
	_ = tcpConn.SetKeepAlive(true)
	_ = tcpConn.SetKeepAlivePeriod(tcpKeepAlivePeriod)
}

func acquireCopyBuffer() []byte {
	return *copyBufferPool.Get().(*[]byte)
}

func releaseCopyBuffer(buf []byte) {
	if cap(buf) < copyBufferSize {
		return
	}
	buf = buf[:copyBufferSize]
	copyBufferPool.Put(&buf)
}
