package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestRunClosesGRPCListenerOnHTTPListenError(t *testing.T) {
	occupiedHTTP, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen HTTP: %v", err)
	}
	defer occupiedHTTP.Close()

	availableGRPC, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen gRPC: %v", err)
	}
	grpcAddress := availableGRPC.Addr().String()
	if err := availableGRPC.Close(); err != nil {
		t.Fatalf("close gRPC probe: %v", err)
	}

	err = run([]string{"-a=" + occupiedHTTP.Addr().String(), "-g=" + grpcAddress, "-f="})
	if err == nil {
		t.Fatal("run() error = nil, want HTTP listen error")
	}

	reopenedGRPC, err := net.Listen("tcp", grpcAddress)
	if err != nil {
		t.Fatalf("gRPC listener was not closed: %v", err)
	}
	_ = reopenedGRPC.Close()
}

func TestRunServerWaitsForActiveRequest(t *testing.T) {
	requestStarted := make(chan struct{})
	releaseRequest := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(requestStarted)
		<-releaseRequest
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := &http.Server{Handler: handler}
	ctx, cancel := context.WithCancel(context.Background())
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- runServer(ctx, server, listener)
	}()

	requestDone := make(chan error, 1)
	go func() {
		response, err := http.Get("http://" + listener.Addr().String())
		if err != nil {
			requestDone <- err
			return
		}
		_, readErr := io.Copy(io.Discard, response.Body)
		closeErr := response.Body.Close()
		requestDone <- errors.Join(readErr, closeErr)
	}()

	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		t.Fatal("request did not start")
	}
	cancel()

	select {
	case err := <-serverDone:
		t.Fatalf("server stopped before request completed: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	close(releaseRequest)
	select {
	case err := <-requestDone:
		if err != nil {
			t.Fatalf("request error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("request did not complete")
	}
	select {
	case err := <-serverDone:
		if err != nil {
			t.Fatalf("runServer() error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("server did not stop")
	}
}

func TestRunServerStopsAfterShutdownTimeout(t *testing.T) {
	requestStarted := make(chan struct{})
	releaseRequest := make(chan struct{})
	var releaseOnce sync.Once
	release := func() {
		releaseOnce.Do(func() {
			close(releaseRequest)
		})
	}
	defer release()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(requestStarted)
		<-releaseRequest
		w.WriteHeader(http.StatusOK)
	})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := &http.Server{Handler: handler}
	ctx, cancel := context.WithCancel(context.Background())
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- runServerWithShutdownTimeout(ctx, server, listener, 20*time.Millisecond)
	}()

	requestDone := make(chan error, 1)
	go func() {
		response, err := http.Get("http://" + listener.Addr().String())
		if err != nil {
			requestDone <- err
			return
		}
		_, readErr := io.Copy(io.Discard, response.Body)
		closeErr := response.Body.Close()
		requestDone <- errors.Join(readErr, closeErr)
	}()

	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		t.Fatal("request did not start")
	}
	cancel()

	select {
	case err := <-serverDone:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("runServer() error = %v, want deadline exceeded", err)
		}
	case <-time.After(time.Second):
		t.Fatal("server did not stop after shutdown timeout")
	}

	release()
	select {
	case err := <-requestDone:
		if err != nil {
			t.Fatalf("request error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("request did not finish after release")
	}
}
