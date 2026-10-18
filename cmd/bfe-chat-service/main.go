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

type runtimeConfig struct{ listen, origin, uds, dsnEnv, migrations string }

func parseRuntimeConfig(args []string) (runtimeConfig, error) {
	flags := flag.NewFlagSet("bfe-chat-service", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	listen := flags.String("listen", "127.0.0.1:8081", "loopback HTTP listener behind HTTPS ingress")
	origin := flags.String("public-origin", "", "public HTTPS origin for DPoP verification")
	uds := flags.String("drive-auth-uds", "", "existing BFE Drive internal authentication Unix socket")
	dsnEnv := flags.String("dsn-env", "BFE_CHAT_DATABASE_URL", "environment variable name with database DSN")
	migrations := flags.String("migrations", "", "approved Chat goose migrations directory")
	if err := flags.Parse(args); err != nil {
		return runtimeConfig{}, errors.New("invalid startup flags")
	}
	if flags.NArg() != 0 {
		return runtimeConfig{}, errors.New("unexpected startup arguments")
	}
	c := runtimeConfig{*listen, *origin, *uds, *dsnEnv, *migrations}
	if c.uds == "" || c.origin == "" || c.migrations == "" || c.dsnEnv == "" {
		return runtimeConfig{}, errors.New("missing runtime configuration")
	}
	host, _, err := net.SplitHostPort(c.listen)
	if err != nil {
		return runtimeConfig{}, errors.New("invalid listen address")
	}
	if host != "127.0.0.1" && host != "::1" {
		return runtimeConfig{}, errors.New("listener must bind loopback")
	}
	return c, nil
}
func validateRuntimeDependencies(c runtimeConfig) error {
	if os.Getenv(c.dsnEnv) == "" {
		return errors.New("missing database credentials")
	}
	info, err := os.Stat(c.uds)
	if err != nil {
		return errors.New("drive authentication UDS unavailable")
	}
	if info.Mode()&os.ModeSocket == 0 {
		return errors.New("drive authentication UDS unavailable")
	}
	return nil
}
func openChatDB(ctx context.Context, c runtimeConfig) (*sql.DB, error) {
	db, err := sql.Open("pgx", os.Getenv(c.dsnEnv))
	if err != nil {
		return nil, errors.New("database initialization failed")
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, errors.New("database unavailable")
	}
	if err := goose.SetDialect("postgres"); err != nil {
		_ = db.Close()
		return nil, errors.New("migration dialect unavailable")
	}
	if err := goose.UpContext(ctx, db, c.migrations); err != nil {
		_ = db.Close()
		return nil, errors.New("database migration failed")
	}
	return db, nil
}
func buildHandler(db *sql.DB, c runtimeConfig) (http.Handler, *centrifuge.Node, error) {
	auth, err := authentication.NewValidator(c.uds, 5*time.Minute)
	if err != nil {
		return nil, nil, errors.New("authentication configuration rejected")
	}
	node, err := centrifuge.New(centrifuge.Config{})
	if err != nil {
		return nil, nil, errors.New("centrifuge initialization failed")
	}
	store := storage.Store{DB: db}
	permissions := authorization.Authorizer{Assignments: store}
	messages := conversations.Service{Auth: auth, Permissions: permissions, Store: store}
	assignmentsSvc := assignments.Service{Auth: auth, Store: store}
	realtimeSvc := realtime.Service{Auth: auth, Permissions: permissions, Assignments: store, Publisher: node}
	handler, err := transport.NewHandler(messages, assignmentsSvc, realtimeSvc, node, strings.TrimSuffix(c.origin, "/")+"/ws", c.origin)
	if err != nil {
		return nil, nil, errors.New("transport configuration rejected")
	}
	return handler, node, nil
}
func run(ctx context.Context, args []string) error {
	c, err := parseRuntimeConfig(args)
	if err != nil {
		return err
	}
	if err := validateRuntimeDependencies(c); err != nil {
		return err
	}
	db, err := openChatDB(ctx, c)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	handler, node, err := buildHandler(db, c)
	if err != nil {
		return err
	}
	return serveRuntime(ctx, c.listen, handler, node)
}
func serveRuntime(ctx context.Context, addr string, handler http.Handler, node *centrifuge.Node) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return errors.New("listener unavailable")
	}
	defer func() { _ = listener.Close() }()
	if err := node.Run(); err != nil {
		return errors.New("centrifuge startup failed")
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
