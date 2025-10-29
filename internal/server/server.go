package server

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/nicexiaonie/number-dispenser/internal/dispenser"
	"github.com/nicexiaonie/number-dispenser/internal/protocol"
	"github.com/nicexiaonie/number-dispenser/internal/storage"
)

// Server represents the number dispenser server
type Server struct {
	addr       string
	listener   net.Listener
	storage    storage.Storage
	dispensers map[string]dispenser.NumberDispenser // 使用接口类型
	factory    *dispenser.DispenserFactory
	mu         sync.RWMutex
	wg         sync.WaitGroup
	shutdown   chan struct{}
}

// NewServer creates a new server
func NewServer(addr string, dataDir string) (*Server, error) {
	st, err := storage.NewFileStorage(dataDir, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	// 创建持久化函数
	persistFunc := func(name string, cfg dispenser.Config, current int64) error {
		return st.Save(name, cfg, current)
	}

	// 创建发号器工厂
	factory := dispenser.NewDispenserFactory(persistFunc)

	s := &Server{
		addr:       addr,
		storage:    st,
		dispensers: make(map[string]dispenser.NumberDispenser),
		factory:    factory,
		shutdown:   make(chan struct{}),
	}

	// Load existing dispensers from storage
	if err := s.loadDispensers(); err != nil {
		return nil, fmt.Errorf("failed to load dispensers: %w", err)
	}

	return s, nil
}

// Start starts the server
func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("failed to start listener: %w", err)
	}

	s.listener = listener
	log.Printf("Number dispenser server listening on %s", s.addr)

	// Handle graceful shutdown
	go s.handleShutdown()

	// Start periodic persistence
	go s.periodicPersist()

	// Accept connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-s.shutdown:
				return nil
			default:
				log.Printf("Error accepting connection: %v", err)
				continue
			}
		}

		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

// Stop stops the server gracefully
func (s *Server) Stop() error {
	close(s.shutdown)

	if s.listener != nil {
		s.listener.Close()
	}

	s.wg.Wait()

	// 优雅关闭所有发号器
	s.mu.Lock()
	for name, d := range s.dispensers {
		if err := d.Shutdown(); err != nil {
			log.Printf("Failed to shutdown dispenser %s: %v", name, err)
		}
	}
	s.mu.Unlock()

	// Final persistence
	return s.persistAll()
}

// handleConnection handles a client connection
func (s *Server) handleConnection(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	reader := protocol.NewReader(conn)
	writer := protocol.NewWriter(conn)

	log.Printf("Client connected: %s", conn.RemoteAddr())

	for {
		select {
		case <-s.shutdown:
			return
		default:
		}

		// Set read deadline to detect client disconnect
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		val, err := reader.ReadValue()
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// Timeout, continue
				continue
			}
			log.Printf("Error reading from client %s: %v", conn.RemoteAddr(), err)
			return
		}

		// Reset deadline after successful read
		conn.SetReadDeadline(time.Time{})

		// Process command
		response := s.processCommand(val)
		if err := writer.WriteValue(response); err != nil {
			log.Printf("Error writing to client %s: %v", conn.RemoteAddr(), err)
			return
		}
	}
}

// processCommand processes a Redis command
func (s *Server) processCommand(val protocol.Value) protocol.Value {
	if val.Type != protocol.Array || len(val.Array) == 0 {
		return protocol.Value{Type: protocol.Error, Str: "ERR invalid command format"}
	}

	// Extract command and arguments
	args := make([]string, len(val.Array))
	for i, v := range val.Array {
		if v.Type == protocol.BulkString {
			args[i] = v.Bulk
		} else {
			return protocol.Value{Type: protocol.Error, Str: "ERR invalid argument type"}
		}
	}

	cmd := args[0]

	switch cmd {
	case "HSET", "hset":
		return s.handleHSet(args[1:])
	case "GET", "get":
		return s.handleGet(args[1:])
	case "DEL", "del":
		return s.handleDel(args[1:])
	case "INFO", "info":
		return s.handleInfo(args[1:])
	case "PING", "ping":
		return protocol.Value{Type: protocol.SimpleString, Str: "PONG"}
	case "QUIT", "quit":
		return protocol.Value{Type: protocol.SimpleString, Str: "OK"}
	default:
		return protocol.Value{Type: protocol.Error, Str: fmt.Sprintf("ERR unknown command '%s'", cmd)}
	}
}

// loadDispensers loads all dispensers from storage
func (s *Server) loadDispensers() error {
	all, err := s.storage.ListAll()
	if err != nil {
		return err
	}

	for name, data := range all {
		// 使用工厂创建发号器
		d, err := s.factory.CreateDispenser(name, data.Config)
		if err != nil {
			log.Printf("Failed to restore dispenser %s: %v", name, err)
			continue
		}
		d.SetCurrent(data.Current)
		s.dispensers[name] = d
		log.Printf("Restored dispenser: %s (type=%d, strategy=%s, current=%d)",
			name, data.Config.Type, data.Config.AutoDisk, data.Current)
	}

	return nil
}

// persistAll saves all dispensers to storage
func (s *Server) persistAll() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for name, d := range s.dispensers {
		if err := s.storage.Save(name, d.GetConfig(), d.GetCurrent()); err != nil {
			log.Printf("Failed to persist dispenser %s: %v", name, err)
		}
	}

	// Flush to disk
	if fs, ok := s.storage.(*storage.FileStorage); ok {
		return fs.Flush()
	}

	return nil
}

// periodicPersist periodically persists dispenser state
func (s *Server) periodicPersist() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := s.persistAll(); err != nil {
				log.Printf("Periodic persist failed: %v", err)
			}
		case <-s.shutdown:
			return
		}
	}
}

// handleShutdown handles graceful shutdown signals
func (s *Server) handleShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("Shutting down server...")
	s.Stop()
}
