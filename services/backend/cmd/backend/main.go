// Command backend serves the API.
//
// It resolves configuration from the environment, builds the concrete adapters the service
// depends on (Firestore repositories, a Google token verifier), and hands them to the service
// package, which owns the routes and business logic.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	fs "cloud.google.com/go/firestore"

	"github.com/edgorman/webapp-template/services/backend/internal/logging"
	"github.com/edgorman/webapp-template/services/backend/internal/repository"
	"github.com/edgorman/webapp-template/services/backend/internal/repository/devauth"
	"github.com/edgorman/webapp-template/services/backend/internal/repository/firestore"
	"github.com/edgorman/webapp-template/services/backend/internal/repository/google"
	"github.com/edgorman/webapp-template/services/backend/internal/service"
)

// Baked in at build time via -ldflags (see Dockerfile); images are never rebuilt for production.
var commit = "unknown"

func main() {
	// Set before anything can log, so every line the process writes - including any a dependency
	// sends through the standard log package, which slog.SetDefault redirects - reaches Cloud
	// Logging as structured JSON with a severity (see internal/logging).
	slog.SetDefault(slog.New(logging.NewHandler(os.Stdout)))

	// run() owns the deferred cleanup; exiting from inside it would skip that via os.Exit.
	if err := run(); err != nil {
		slog.Error("backend stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	environment := envOr("ENVIRONMENT", "development")
	port := envOr("PORT", "8080")

	verifier, err := tokenVerifier(environment, os.Getenv("DEV_AUTH") == "true", os.Getenv("GOOGLE_CLIENT_ID"))
	if err != nil {
		return err
	}

	ctx := context.Background()

	client, err := fs.NewClient(ctx, fs.DetectProjectID)
	if err != nil {
		return fmt.Errorf("firestore client: %w", err)
	}
	defer client.Close()

	api := service.New(
		service.Config{
			Environment:   environment,
			Commit:        commit,
			AllowedOrigin: os.Getenv("CORS_ALLOWED_ORIGIN"),
		},
		firestore.NewNoteRepository(client),
		firestore.NewUserRepository(client),
		verifier,
	)

	// The server is built explicitly rather than handed to http.ListenAndServe, which supplies a
	// zero-value http.Server: one with no timeouts of any kind. A connection that opens and then
	// sends nothing, or dribbles a request a byte at a time, would hold a goroutine and a file
	// descriptor until the peer gave up - which costs the sender nothing and is the cheapest denial
	// of service there is. Cloud Run bounds the request it forwards but not the connection beneath
	// it, so the bound has to be here.
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	slog.Info("backend listening", "port", port, "environment", environment, "commit", commit)
	return server.ListenAndServe()
}

// tokenVerifier picks how ID tokens are checked. DEV_AUTH swaps Google sign-in for fixed development
// tokens (see internal/repository/devauth), which would let anybody sign in as anybody, so it is
// refused outside ENVIRONMENT=local rather than trusted to be left unset everywhere else.
func tokenVerifier(environment string, devAuth bool, googleClientID string) (repository.TokenVerifier, error) {
	if devAuth {
		if environment != "local" {
			return nil, fmt.Errorf("DEV_AUTH is only allowed with ENVIRONMENT=local, not %q", environment)
		}
		slog.Warn("DEV_AUTH is set, so the development token signs anybody in", "token", devauth.Token)
		return devauth.TokenVerifier{}, nil
	}
	// The OAuth 2.0 client ID ID tokens must be minted for; unset means no request can authenticate.
	if googleClientID == "" {
		slog.Warn("GOOGLE_CLIENT_ID is unset, so no request can authenticate")
	}
	return google.NewTokenVerifier(googleClientID), nil
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
