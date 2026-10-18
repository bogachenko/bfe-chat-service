package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bogachenko/bfe-chat-service/internal/assignments"
	"github.com/bogachenko/bfe-chat-service/internal/authentication"
	"github.com/bogachenko/bfe-chat-service/internal/authorization"
	"github.com/bogachenko/bfe-chat-service/internal/conversations"
	"github.com/bogachenko/bfe-chat-service/internal/realtime"
	"github.com/bogachenko/bfe-chat-service/internal/storage"
	"github.com/bogachenko/bfe-chat-service/internal/transport"
	"github.com/centrifugal/centrifuge"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func main() {
	log.SetFlags(0)
	if err := run(context.Background(), os.Args[1:]); err != nil {
		// Never log configuration values, DSN or errors from external transports.
		log.Print("bfe-chat-service: startup or runtime failed")
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("bfe-chat-service", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	listen := flags.String("listen", "127.0.0.1:8081", "loopback HTTP listener behind HTTPS ingress")
	origin := flags.String("public-origin", "", "public HTTPS origin for DPoP verification")
	uds := flags.String("drive-auth-uds", "", "existing BFE Drive internal authentication Unix socket")
	dsnEnv := flags.String("dsn-env", "BFE_CHAT_DATABASE_URL", "name of environment variable containing PostgreSQL DSN")
	migrations := flags.String("migrations", "", "path to approved Chat Service goose migration files")
	if err := flags.Parse(args); err != nil {
		return errors.New("invalid startup flags")
	}
	if flags.NArg() != 0 || *uds == "" || *origin == "" || *migrations == "" || *dsnEnv == "" {
		return errors.New("missing runtime configuration")
	}
	host, _, err := net.SplitHostPort(*listen)
	if err != nil || host != "127.0.0.1" && host != "::1" {
		return errors.New("listener must bind loopback")
	}
	dbURL := os.Getenv(*dsnEnv)
	if dbURL == "" {
		return errors.New("missing database credentials")
	}
	info, err := os.Stat(*uds)
	if err != nil || info.Mode()&os.ModeSocket == 0 {
		return errors.New("Drive authentication UDS unavailable")
	}
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		return errors.New("database initialization failed")
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return errors.New("database unavailable")
	}
	if err := goose.SetDialect("postgres"); err != nil {
		return errors.New("migration dialect unavailable")
	}
	if err := goose.UpContext(ctx, db, *migrations); err != nil {
		return errors.New("database migration failed")
	}
	auth, err := authentication.NewValidator(*uds, 5*time.Minute)
	if err != nil {
		return errors.New("authentication configuration rejected")
	}
	node, err := centrifuge.New(centrifuge.Config{})
	if err != nil {
		return errors.New("Centrifuge initialization failed")
	}
	store := storage.Store{DB: db}
	permissions := authorization.Authorizer{Assignments: store}
	messages := conversations.Service{Auth: auth, Permissions: permissions, Store: store}
	assignmentsSvc := assignments.Service{Auth: auth, Store: store}
	realtimeSvc := realtime.Service{Auth: auth, Permissions: permissions, Assignments: store, Publisher: node}
	handler, err := transport.NewHandler(messages, assignmentsSvc, realtimeSvc, node, strings.TrimSuffix(*origin, "/")+"/ws", *origin)
	if err != nil {
		return errors.New("transport configuration rejected")
	}
	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		return errors.New("listener unavailable")
	}
	defer listener.Close()
	if err := node.Run(); err != nil {
		return errors.New("Centrifuge startup failed")
	}
	defer func() {
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = node.Shutdown(shutdown)
	}()
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second, MaxHeaderBytes: 16 << 10}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	signals, stop := signal.NotifyContext(ctx, syscall.SIGTERM, os.Interrupt)
	defer stop()
	select {
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("listener stopped: %w", err)
	case <-signals.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdown)
	}
}
