// Copyright (c) 2013 Project Iris. All rights reserved.
//
// The current language binding is an official support library of the Iris
// cloud messaging framework, and as such, the same licensing terms apply.
// For details please see http://iris.karalabe.com/downloads#License

package tests

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/project-iris/iris/pool"
	"gopkg.in/project-iris/iris-go.v0"
)

// Service handler for the broadcast tests.
type broadcastTestHandler struct {
	conn     *iris.Connection
	delivers chan []byte
}

func (b *broadcastTestHandler) Init(conn *iris.Connection) error         { b.conn = conn; return nil }
func (b *broadcastTestHandler) HandleBroadcast(msg []byte)               { b.delivers <- msg }
func (b *broadcastTestHandler) HandleRequest(req []byte) ([]byte, error) { panic("not implemented") }
func (b *broadcastTestHandler) HandleTunnel(tun *iris.Tunnel)            { panic("not implemented") }
func (b *broadcastTestHandler) HandleDrop(reason error)                  { panic("not implemented") }

// Tests multiple concurrent client and service broadcasts.
func TestBroadcast(t *testing.T) {
	// Test specific configurations
	conf := struct {
		clients  int
		servers  int
		messages int
	}{25, 25, 25}

	barrier := newBarrier(conf.clients + conf.servers)

	// Start up the concurrent broadcasting clients
	for i := 0; i < conf.clients; i++ {
		go func(client int) {
			// Connect to the local relay
			conn, err := iris.Connect(config.relay)
			if err != nil {
				barrier.Exit(fmt.Errorf("connection failed: %v", err))
				return
			}
			defer conn.Close()
			barrier.Sync()

			// Broadcast to the whole service cluster
			for i := 0; i < conf.messages; i++ {
				message := fmt.Sprintf("client #%d, broadcast %d", client, i)
				if err := conn.Broadcast(config.cluster, []byte(message)); err != nil {
					barrier.Exit(fmt.Errorf("client broadcast failed: %v", err))
					return
				}
			}
			barrier.Sync() // Make sure we don't terminate prematurely
			barrier.Exit(nil)
		}(i)
	}
	// Start up the concurrent broadcast services
	for i := 0; i < conf.servers; i++ {
		go func(server int) {
			// Create the service handler
			handler := &broadcastTestHandler{
				delivers: make(chan []byte, (conf.clients+conf.servers)*conf.messages),
			}
			// Register a new service to the relay
			serv, err := iris.Register(config.relay, config.cluster, handler)
			if err != nil {
				barrier.Exit(fmt.Errorf("registration failed: %v", err))
				return
			}
			defer serv.Unregister()
			barrier.Sync()

			// Broadcast to the whole service cluster
			for i := 0; i < conf.messages; i++ {
				message := fmt.Sprintf("server #%d, broadcast %d", server, i)
				if err := handler.conn.Broadcast(config.cluster, []byte(message)); err != nil {
					barrier.Exit(fmt.Errorf("server broadcast failed: %v", err))
					return
				}
			}
			barrier.Sync()

			// Retrieve all the arrived broadcasts
			messages := make(map[string]struct{})
			for i := 0; i < (conf.clients+conf.servers)*conf.messages; i++ {
				select {
				case msg := <-handler.delivers:
					messages[string(msg)] = struct{}{}
				case <-time.After(time.Second):
					barrier.Exit(errors.New("broadcast receive timeout"))
					return
				}
			}
			// Verify all the individual broadcasts
			for i := 0; i < conf.clients; i++ {
				for j := 0; j < conf.messages; j++ {
					msg := fmt.Sprintf("client #%d, broadcast %d", i, j)
					if _, ok := messages[msg]; !ok {
						barrier.Exit(fmt.Errorf("broadcast not found: %s", msg))
						return
					}
					delete(messages, msg)
				}
			}
			for i := 0; i < conf.servers; i++ {
				for j := 0; j < conf.messages; j++ {
					msg := fmt.Sprintf("server #%d, broadcast %d", i, j)
					if _, ok := messages[msg]; !ok {
						barrier.Exit(fmt.Errorf("broadcast not found: %s", msg))
						return
					}
					delete(messages, msg)
				}
			}
			barrier.Exit(nil)
		}(i)
	}
	// Schedule the parallel operations
	if errs := barrier.Wait(); len(errs) != 0 {
		t.Fatalf("startup phase failed: %v.", errs)
	}
	if errs := barrier.Wait(); len(errs) != 0 {
		t.Fatalf("broadcasting phase failed: %v.", errs)
	}
	if errs := barrier.Wait(); len(errs) != 0 {
		t.Fatalf("verification phase failed: %v.", errs)
	}
}

// Benchmarks broadcasting a single message.
func BenchmarkBroadcastLatency(b *testing.B) {
	// Create the service handler
	handler := &broadcastTestHandler{
		delivers: make(chan []byte, b.N),
	}
	// Register a new service to the relay
	serv, err := iris.Register(config.relay, config.cluster, handler)
	if err != nil {
		b.Fatalf("registration failed: %v", err)
	}
	defer serv.Unregister()

	// Reset timer and benchmark the message transfer
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler.conn.Broadcast(config.cluster, []byte{byte(i)})
		<-handler.delivers
	}
}

// Benchmarks broadcasting a stream of messages.
func BenchmarkBroadcastThroughput1Threads(b *testing.B) {
	benchmarkBroadcastThroughput(1, b)
}

func BenchmarkBroadcastThroughput2Threads(b *testing.B) {
	benchmarkBroadcastThroughput(2, b)
}

func BenchmarkBroadcastThroughput4Threads(b *testing.B) {
	benchmarkBroadcastThroughput(4, b)
}

func BenchmarkBroadcastThroughput8Threads(b *testing.B) {
	benchmarkBroadcastThroughput(8, b)
}

func BenchmarkBroadcastThroughput16Threads(b *testing.B) {
	benchmarkBroadcastThroughput(16, b)
}

func BenchmarkBroadcastThroughput32Threads(b *testing.B) {
	benchmarkBroadcastThroughput(32, b)
}

func BenchmarkBroadcastThroughput64Threads(b *testing.B) {
	benchmarkBroadcastThroughput(64, b)
}

func BenchmarkBroadcastThroughput128Threads(b *testing.B) {
	benchmarkBroadcastThroughput(128, b)
}

func benchmarkBroadcastThroughput(threads int, b *testing.B) {
	// Create the service handler
	handler := &broadcastTestHandler{
		delivers: make(chan []byte, b.N),
	}
	// Register a new service to the relay
	serv, err := iris.Register(config.relay, config.cluster, handler)
	if err != nil {
		b.Fatalf("registration failed: %v", err)
	}
	defer serv.Unregister()

	// Create the thread pool with the concurrent broadcasts
	workers := pool.NewThreadPool(threads)
	for i := 0; i < b.N; i++ {
		workers.Schedule(func() {
			if err := handler.conn.Broadcast(config.cluster, []byte{byte(i)}); err != nil {
				b.Fatalf("broadcast failed: %v.", err)
			}
		})
	}
	// Reset timer and benchmark the message transfer
	b.ResetTimer()
	workers.Start()
	for i := 0; i < b.N; i++ {
		<-handler.delivers
	}
	b.StopTimer()
	workers.Terminate(true)
}
