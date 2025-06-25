package hl7

import (
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"
)

// ConnectionPool manages a pool of reusable MLLP connections with automatic recovery
type ConnectionPool struct {
	host        string
	port        int
	maxConns    int
	timeout     time.Duration
	connections chan *poolConn
	mu          sync.Mutex
	closed      bool
}

type poolConn struct {
	conn     net.Conn
	lastUsed time.Time
	pool     *ConnectionPool
	inUse    bool
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(host string, port int, maxConns int) *ConnectionPool {
	if maxConns <= 0 {
		maxConns = 5
	}

	pool := &ConnectionPool{
		host:        host,
		port:        port,
		maxConns:    maxConns,
		timeout:     30 * time.Second,
		connections: make(chan *poolConn, maxConns),
		closed:      false,
	}

	// Start health check routine
	go pool.healthCheck()

	return pool
}

// Get retrieves a connection from the pool or creates a new one
func (p *ConnectionPool) Get() (net.Conn, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, fmt.Errorf("connection pool is closed")
	}
	p.mu.Unlock()

	// Try to get an existing connection
	select {
	case pc := <-p.connections:
		// Test if connection is still alive
		if p.isConnectionAlive(pc.conn) {
			pc.inUse = true
			pc.lastUsed = time.Now()
			return &wrappedConn{Conn: pc.conn, pc: pc}, nil
		}
		// Connection is dead, close it
		pc.conn.Close()
	default:
		// No connections available
	}

	// Create new connection
	addr := fmt.Sprintf("%s:%d", p.host, p.port)
	conn, err := net.DialTimeout("tcp", addr, p.timeout)
	if err != nil {
		return nil, fmt.Errorf("bağlantı hatası %s: %w", addr, err)
	}

	// Set keep-alive
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
	}

	pc := &poolConn{
		conn:     conn,
		lastUsed: time.Now(),
		pool:     p,
		inUse:    true,
	}

	slog.Debug("Yeni bağlantı oluşturuldu", "address", addr)

	return &wrappedConn{Conn: conn, pc: pc}, nil
}

// Put returns a connection to the pool
func (p *ConnectionPool) Put(pc *poolConn) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		pc.conn.Close()
		return
	}

	pc.inUse = false
	pc.lastUsed = time.Now()

	// Try to return to pool
	select {
	case p.connections <- pc:
		// Successfully returned to pool
	default:
		// Pool is full, close the connection
		pc.conn.Close()
	}
}

// Close closes all connections in the pool
func (p *ConnectionPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true
	close(p.connections)

	// Close all connections
	for pc := range p.connections {
		pc.conn.Close()
	}

	return nil
}

// isConnectionAlive checks if a connection is still usable
func (p *ConnectionPool) isConnectionAlive(conn net.Conn) bool {
	// Set a very short deadline
	conn.SetReadDeadline(time.Now().Add(1 * time.Millisecond))

	// Try to read one byte
	one := make([]byte, 1)
	_, err := conn.Read(one)

	// Reset the deadline
	conn.SetReadDeadline(time.Time{})

	// If we got a timeout, the connection is probably still alive
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}

	// Any other error means the connection is dead
	return err == nil
}

// healthCheck periodically checks and removes stale connections
func (p *ConnectionPool) healthCheck() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		p.mu.Lock()
		if p.closed {
			p.mu.Unlock()
			return
		}
		p.mu.Unlock()

		// Check connections
		var healthy []*poolConn

		for {
			select {
			case pc := <-p.connections:
				if pc.inUse {
					healthy = append(healthy, pc)
					continue
				}

				// Check if connection is stale (unused for > 5 minutes)
				if time.Since(pc.lastUsed) > 5*time.Minute {
					pc.conn.Close()
					slog.Debug("Eski bağlantı kapatıldı", "age", time.Since(pc.lastUsed))
					continue
				}

				// Check if connection is alive
				if p.isConnectionAlive(pc.conn) {
					healthy = append(healthy, pc)
				} else {
					pc.conn.Close()
					slog.Debug("Ölü bağlantı kapatıldı")
				}
			default:
				// No more connections to check
				goto done
			}
		}

	done:
		// Return healthy connections to pool
		for _, pc := range healthy {
			select {
			case p.connections <- pc:
			default:
				pc.conn.Close()
			}
		}
	}
}

// wrappedConn wraps a connection to return it to the pool when closed
type wrappedConn struct {
	net.Conn
	pc     *poolConn
	closed bool
	mu     sync.Mutex
}

func (w *wrappedConn) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}

	w.closed = true

	// Return to pool instead of closing
	w.pc.pool.Put(w.pc)
	return nil
}
