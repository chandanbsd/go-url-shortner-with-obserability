package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"boot.dev/linko/internal/store"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	httpPort := flag.Int("port", 8899, "port to listen on")
	dataDir := flag.String("data", "./data", "directory to store data")
	flag.Parse()

	status := run(ctx, cancel, *httpPort, *dataDir)
	cancel()
	os.Exit(status)
}

func initializeLogger() (*log.Logger, error) {
	LINKO_LOG_FILE := os.Getenv("LINKO_LOG_FILE")

	writer := io.Writer(os.Stderr)

	if LINKO_LOG_FILE != "" {
		linkoAccessLogFile, err := os.OpenFile(LINKO_LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, fmt.Errorf("Failed to initialize logger %v", err)
		}
		defer linkoAccessLogFile.Close()

		bufferedFile := bufio.NewWriterSize(linkoAccessLogFile, 8192)


		writer = io.MultiWriter(os.Stderr, bufferedFile)
	}

	return log.New(writer, "", 0), nil
}

func run(ctx context.Context, cancel context.CancelFunc, httpPort int, dataDir string) int {

	customLogger, err := initializeLogger()
	if err != nil {
		fmt.Println(err)
		return 1;
	}

	st, err := store.New(dataDir, customLogger)
	if err != nil {
		customLogger.Printf("failed to create store: %v", err)
		return 1
	}
	s := newServer(*st, httpPort, cancel, customLogger)
	var serverErr error
	go func() {
		serverErr = s.start()
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.shutdown(shutdownCtx); err != nil {
		customLogger.Printf("failed to shutdown server: %v", err)
		return 1
	}
	if serverErr != nil {
		customLogger.Printf("server error: %v", serverErr)
		return 1
	}
	return 0
}
