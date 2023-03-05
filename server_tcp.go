package modbus

import (
	"context"
	"errors"
	"log"
	"net"
	"sync"
	"time"
)

type ErrorLevel uint8

const (
	Silent ErrorLevel = iota
	INFO
	ERROR
	DEBUG
)

type Server struct {
	address string
	serve   func(conn *Conn)

	logLevel ErrorLevel

	// 打印连接数量的时间间隔，默认5分钟
	interval time.Duration
}

func NewServer(address string) *Server {
	return &Server{
		address:  address,
		logLevel: ERROR,
		interval: 5 * time.Minute,
	}
}

func (s *Server) SetServe(serve func(conn *Conn)) {
	s.serve = serve
}

func (s *Server) SetInterval(interval time.Duration) {
	s.interval = interval
}

func (s *Server) SetLogLevel(logLevel ErrorLevel) {
	s.logLevel = logLevel
}

func (s *Server) ListenAndServe() error {
	listener, err := net.Listen("tcp", s.address)
	if err != nil {
		return err
	}

	defer listener.Close()

	var counter AtomicCounter

	if s.logLevel >= INFO {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			ticker := time.NewTicker(s.interval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					ticker.Stop()
					ticker = time.NewTicker(s.interval)
					log.Printf("INFO connections: %v\n", counter.Load())
				}
			}
		}()
	}
	for {
		rwc, err := listener.Accept()
		if err != nil {
			return err
		}

		counter.Add(1)

		go func() {
			s.serve(&Conn{rwc: rwc, server: s})
			_ = rwc.Close()
			counter.Add(-1)
		}()
	}
}

type Conn struct {
	rwc    net.Conn
	server *Server
	mu     sync.Mutex
	ch     chan *Frame
}

func (c *Conn) Read(timeout time.Duration, size int) ([]byte, error) {
	_ = c.rwc.SetDeadline(time.Now().Add(timeout))

	defer c.rwc.SetDeadline(time.Time{})

	buf := make([]byte, size)

	l, err := c.rwc.Read(buf)
	if err != nil {
		return nil, err
	}

	if c.server.logLevel == DEBUG {
		log.Printf("DEBUG %v read: % x\n", c.rwc.RemoteAddr(), buf[:l])
	}

	return buf[:l], nil
}

func (c *Conn) ReadFrame(timeout time.Duration, size int) (*Frame, error) {
	_ = c.rwc.SetDeadline(time.Now().Add(timeout))

	defer c.rwc.SetDeadline(time.Time{})

	buf := make([]byte, size)

	l, err := c.rwc.Read(buf)
	if err != nil {
		return nil, err
	}

	if c.server.logLevel == DEBUG {
		log.Printf("DEBUG %v read: % x\n", c.rwc.RemoteAddr(), buf[:l])
	}

	return NewFrame(buf[:l])
}

func (c *Conn) Write(timeout time.Duration, buf []byte) error {
	// 控制写入频率，减少粘包
	defer time.Sleep(100)
	_ = c.rwc.SetDeadline(time.Now().Add(timeout))

	defer c.rwc.SetDeadline(time.Time{})

	if c.server.logLevel == DEBUG {
		log.Printf("DEBUG %v write: % x\n", c.rwc.RemoteAddr(), buf)
	}

	_, err := c.rwc.Write(buf)

	return err
}

func (c *Conn) WriteFrame(timeout time.Duration, frame *Frame) error {
	// 控制写入频率，减少粘包
	defer time.Sleep(100)
	buf := frame.Bytes()
	_ = c.rwc.SetDeadline(time.Now().Add(timeout))

	defer c.rwc.SetDeadline(time.Time{})

	if c.server.logLevel == DEBUG {
		log.Printf("DEBUG %v write: % x\n", c.rwc.RemoteAddr(), buf)
	}

	_, err := c.rwc.Write(buf)

	return err
}

func (c *Conn) Close() error {
	return c.rwc.Close()
}

func (c *Conn) Addr() net.Addr {
	return c.rwc.RemoteAddr()
}

func (c *Conn) NewChan() {
	c.ch = make(chan *Frame)
}
func (c *Conn) CloseChan() {
	close(c.ch)
	c.ch = nil
}

func (c *Conn) Store(ctx context.Context, frame *Frame) error {
	if c.ch == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return errors.New("timeout")
	case c.ch <- frame:

	}
	return nil
}

func (c *Conn) Load(ctx context.Context) (*Frame, error) {
	if c.ch == nil {
		return nil, errors.New("nil chan")
	}
	select {
	case <-ctx.Done():
		return nil, errors.New("timeout")
	case frame, ok := <-c.ch:
		if ok {
			return frame, nil
		}
		return nil, errors.New("channel closed")
	}
}

func (c *Conn) Lock() {
	c.mu.Lock()
}

func (c *Conn) Unlock() {
	c.mu.Unlock()
}
