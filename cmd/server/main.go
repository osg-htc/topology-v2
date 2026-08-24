// Command server is the topology webapp entrypoint. It loads configuration,
// bootstraps the master key, connects to Postgres, runs migrations, wires the
// router (with the embedded SPA), and serves HTTP with graceful shutdown.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/bbockelm/topology-v2/internal/config"
	"github.com/bbockelm/topology-v2/internal/db"
	"github.com/bbockelm/topology-v2/internal/githistory"
	"github.com/bbockelm/topology-v2/internal/router"
	"github.com/bbockelm/topology-v2/internal/storage"
	"github.com/bbockelm/topology-v2/internal/topology"
	"github.com/bbockelm/topology-v2/internal/version"
)

func main() {
	// Subcommand dispatch. With no args, run the HTTP server.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "migrate":
			if err := runMigrate(os.Args[2:]); err != nil {
				log.Fatal().Err(err).Msg("migrate failed")
			}
			return
		case "import-tree":
			if err := runImportTree(os.Args[2:]); err != nil {
				log.Fatal().Err(err).Msg("import-tree failed")
			}
			return
		case "backfill-contacts":
			if err := runBackfillContacts(); err != nil {
				log.Fatal().Err(err).Msg("backfill-contacts failed")
			}
			return
		case "import-history":
			if err := runImportHistory(os.Args[2:]); err != nil {
				log.Fatal().Err(err).Msg("import-history failed")
			}
			return
		case "version":
			log.Info().Str("version", version.Version).Str("commit", version.Commit).Msg("topology")
			return
		}
	}
	if err := run(); err != nil {
		log.Fatal().Err(err).Msg("server exited with error")
	}
}

// runImportTree handles `topology-server import-tree <topology-root>`, loading a
// topology YAML tree (facilities/sites/RGs + services/support-centers) into the
// database. Useful for bootstrapping a dev instance and for restore.
func runImportTree(args []string) error {
	if len(args) < 1 {
		return errors.New("usage: import-tree <topology-root>")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.DatabaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	if err := db.RunMigrations(cfg.DatabaseURL); err != nil {
		return err
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	// Import the whole repo (tree + VOs + projects) when siblings are present.
	if err := topology.ImportRepo(ctx, db.New(pool), args[0]); err != nil {
		return err
	}
	log.Info().Str("root", args[0]).Msg("topology repo imported")
	return nil
}

// runBackfillContacts bootstraps provisioned users from existing contacts and
// links resource_contacts.user_id (idempotent; for data imported before the
// contacts-as-users change).
func runBackfillContacts() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.DatabaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	linked, err := db.New(pool).BackfillContactUsers(ctx)
	if err != nil {
		return err
	}
	log.Info().Int("linked", linked).Msg("contact users backfilled")
	return nil
}

// runImportHistory handles `topology-server import-history <repo-root> <ref>
// [since] [limit]`, replaying a v1 topology repo's git history into
// change_proposals -- see internal/githistory's package doc. Must run after
// import-tree has already loaded the current snapshot. since (a date like
// "2019-01-01") and limit (stop after writing this many new proposals, for
// a cheap incremental run against a slice of history) are both optional and
// order-independent -- whichever of the two trailing args looks like a date
// is since, whichever looks like a plain integer is limit.
func runImportHistory(args []string) error {
	if len(args) < 2 {
		return errors.New("usage: import-history <repo-root> <ref> [since] [limit]")
	}
	since := ""
	limit := 0
	for _, a := range args[2:] {
		if n, err := strconv.Atoi(a); err == nil {
			limit = n
			continue
		}
		since = a
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.DatabaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	if err := db.RunMigrations(cfg.DatabaseURL); err != nil {
		return err
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	result, err := githistory.Run(ctx, db.New(pool), githistory.Options{
		RepoPath: args[0], Ref: args[1], Since: since, Limit: limit,
	})
	if err != nil {
		return err
	}
	log.Info().
		Int("commits_considered", result.CommitsConsidered).
		Int("commits_skipped", result.CommitsSkipped).
		Int("proposals_written", result.ProposalsWritten).
		Msg("history import complete")
	return nil
}

// runMigrate handles `topology-server migrate [up|status]`.
func runMigrate(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.DatabaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	action := "up"
	if len(args) > 0 {
		action = args[0]
	}
	switch action {
	case "up":
		return db.RunMigrations(cfg.DatabaseURL)
	case "status":
		return db.MigrationStatus(cfg.DatabaseURL)
	default:
		return errors.New("unknown migrate action: " + action)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := newLogger(cfg)
	log.Logger = logger

	if err := cfg.ValidateServer(); err != nil {
		return err
	}
	if err := cfg.EnsureMasterKey(); err != nil {
		return err
	}

	ctx := context.Background()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := db.RunMigrations(cfg.DatabaseURL); err != nil {
		return err
	}
	logger.Info().Msg("migrations applied")

	queries := db.New(pool)

	// S3 is optional in development; a nil store is tolerated by handlers that
	// don't require it.
	var store *storage.Store
	if cfg.S3Bucket != "" {
		store, err = storage.New(ctx, storage.Options{
			Endpoint:     cfg.S3Endpoint,
			Region:       cfg.S3Region,
			Bucket:       cfg.S3Bucket,
			AccessKey:    cfg.S3AccessKey,
			SecretKey:    cfg.S3SecretKey,
			UsePathStyle: cfg.S3UsePathStyle,
		})
		if err != nil {
			return err
		}
		logger.Info().Str("bucket", cfg.S3Bucket).Msg("object storage ready")
	}

	r, _, err := router.New(cfg, queries, store, logger)
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Graceful shutdown on SIGINT/SIGTERM.
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		logger.Info().Msg("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	logger.Info().
		Str("version", version.Version).
		Str("commit", version.Commit).
		Str("addr", srv.Addr).
		Str("env", cfg.AppEnv).
		Msg("topology server listening")

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// newLogger builds a zerolog logger: pretty console in dev, JSON in production.
func newLogger(cfg *config.Config) zerolog.Logger {
	if cfg.IsProduction() {
		return zerolog.New(os.Stdout).With().Timestamp().Logger()
	}
	return zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.Kitchen}).
		With().Timestamp().Logger()
}
