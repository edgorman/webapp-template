// Package service configures the API server, its routes, and the business logic that spans more
// than one entity.
package service

import (
	"log/slog"
	"net/http"

	"github.com/edgorman/webapp-template/services/backend/internal/repository"
)

// Config holds the deployment-specific values the API reports or enforces. Everything here is
// resolved by the caller (see cmd/backend) rather than read from the environment in-place.
type Config struct {
	// Environment is the deployment name reported by /debug (e.g. stag, prod).
	Environment string
	// Commit is the git SHA the running binary was built from.
	Commit string
	// AllowedOrigin is the frontend origin permitted to call this API from a browser. Empty
	// disables CORS headers entirely.
	AllowedOrigin string
	// Logger is where the service writes what an operator reads - failures it answers the caller
	// about only vaguely. Nil means slog.Default(), which cmd/backend points at Cloud Logging's
	// JSON shape.
	Logger *slog.Logger
}

// Service owns the API's dependencies and serves its routes.
type Service struct {
	cfg      Config
	notes    repository.NoteRepository
	users    repository.UserRepository
	verifier repository.TokenVerifier
	// The rate limiters live on the Service rather than being built in Handler(), so a budget is
	// spent by the service that served the request rather than by the handler tree - two calls to
	// Handler() must not hand a caller two budgets. See ratelimit.go for what each one bounds.
	ipLimiter     *rateLimiter
	callerLimiter *rateLimiter
}

func New(
	cfg Config,
	notes repository.NoteRepository,
	users repository.UserRepository,
	verifier repository.TokenVerifier,
) *Service {
	return &Service{
		cfg:           cfg,
		notes:         notes,
		users:         users,
		verifier:      verifier,
		ipLimiter:     newRateLimiter(requestsPerIP),
		callerLimiter: newRateLimiter(requestsPerCaller),
	}
}

// logger is the configured Logger, or the process default when none was given.
func (s *Service) logger() *slog.Logger {
	if s.cfg.Logger != nil {
		return s.cfg.Logger
	}
	return slog.Default()
}

// Handler returns the fully-wired API, ready to serve.
func (s *Service) Handler() http.Handler {
	// Every write requires a verified caller, since it always resolves to a specific owner. The
	// per-account budget sits inside the verification rather than in front of it, because the
	// account it meters is not known until the credential has been checked; an unverified flood is
	// already bounded by the per-IP budget below, which is what keeps it from reaching the
	// verifier in the first place.
	authed := func(h http.HandlerFunc) http.Handler {
		return requireAuth(s.verifier, rateLimited(s.callerLimiter, callerKey, h))
	}
	// Reads admit anonymous callers. Visibility is enforced downstream: entity.Note.CanBeReadBy
	// treats the zero Caller as seeing only public notes, and a profile has nothing to hide.
	optional := func(h http.HandlerFunc) http.Handler {
		return optionalAuth(s.verifier, h)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.Debug)
	mux.HandleFunc("/debug", s.Debug)
	// The example resource. A new one is registered the same way: reads optional, writes authed,
	// and a line for each action in entity/access.go's policy table.
	mux.Handle("GET /notes", optional(s.ListNotes))
	mux.Handle("GET /notes/{id}", optional(s.GetNote))
	mux.Handle("POST /notes", authed(s.CreateNote))
	mux.Handle("PUT /notes/{id}", authed(s.UpdateNote))
	mux.Handle("DELETE /notes/{id}", authed(s.DeleteNote))
	// A profile is addressed by its username, never by the Google `sub` it is keyed by, so the id
	// stays an internal detail rather than a public handle. "me" is the one exception, for a client
	// that holds a credential but does not yet know which name it was given; ServeMux prefers that
	// literal over the wildcard, so it wins without any ordering care here.
	mux.Handle("GET /users/me", authed(s.GetCurrentUser))
	mux.Handle("PUT /users/me", authed(s.PutUser))
	mux.Handle("DELETE /users/me", authed(s.DeleteUser))
	mux.Handle("GET /users/{username}", optional(s.GetUser))

	// The per-IP budget wraps the whole mux, so it applies to the routes that admit anonymous
	// callers too - the ones with no account to meter - and to a request for a path that does not
	// exist, which a per-route wrapper would never see.
	//
	// CORS wraps that in turn rather than individual routes: routes are registered under a
	// specific method, so ServeMux would 405 an OPTIONS preflight before a per-route wrapper ran.
	// Being outermost also means a preflight is answered without spending a token, and that a 429
	// still carries the CORS headers, without which the browser would show the frontend a network
	// error instead of the reason it was refused.
	return withCORS(s.cfg.AllowedOrigin, rateLimited(s.ipLimiter, clientIP, mux.ServeHTTP))
}
