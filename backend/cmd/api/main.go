package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"filemepls/internal/adapters/bcrypt"
	httpadapter "filemepls/internal/adapters/http"
	"filemepls/internal/adapters/localfs"
	"filemepls/internal/adapters/oauth"
	"filemepls/internal/adapters/postgres"
	"filemepls/internal/ports"
	"filemepls/internal/usecase"
)

func main() {
	cfg := loadConfig()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := postgres.RunMigrations(cfg.DatabaseURL); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	pool, err := postgres.NewPool(ctx, postgres.Config{DSN: cfg.DatabaseURL})
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer pool.Close()

	userRepo := postgres.NewUserRepository(pool)
	blobRepo := postgres.NewBlobRepository(pool)
	fileRepo := postgres.NewFileRepository(pool)
	folderRepo := postgres.NewFolderRepository(pool)
	shareRepo := postgres.NewShareRepository(pool)
	accessGrantRepo := postgres.NewAccessGrantRepository(pool)

	storage, err := localfs.New(cfg.StorageRoot)
	if err != nil {
		log.Fatalf("init storage: %v", err)
	}

	hasher := bcrypt.New()

	authProviders := []ports.AuthProvider{
		oauth.NewGitHubProvider(cfg.GitHubClientID, cfg.GitHubClientSecret, cfg.GitHubRedirectURI),
		oauth.NewGoogleProvider(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURI),
	}

	fileService := usecase.NewFileService(fileRepo, blobRepo, folderRepo, accessGrantRepo, storage, cfg.MaxUploadSize, cfg.AllowedMimes)
	folderService := usecase.NewFolderService(folderRepo, fileRepo, fileService, accessGrantRepo, storage)
	shareService := usecase.NewShareService(fileRepo, folderRepo, shareRepo, storage, hasher)
	permissionService := usecase.NewPermissionService(fileRepo, folderRepo, accessGrantRepo, userRepo)
	authService := usecase.NewAuthService(userRepo, authProviders, []byte(cfg.JWTSecret), cfg.JWTTTL)

	router := httpadapter.NewRouter(httpadapter.Deps{
		Files:           fileService,
		Folders:         folderService,
		Shares:          shareService,
		Permissions:     permissionService,
		Auth:            authService,
		AllowedOrigins:  cfg.CORSAllowedOrigins,
		FrontendBaseURL: cfg.FrontendBaseURL,
		DefaultLocale:   cfg.DefaultLocale,
		JWTTTL:          cfg.JWTTTL,
		CookieDomain:    cfg.CookieDomain,
	})

	srv := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: router,
		// ReadHeaderTimeout (not ReadTimeout) guards against slow/stalled
		// header reads without capping how long reading the request BODY
		// may take — uploads have no size limit (MAX_UPLOAD_SIZE<=0 is
		// supported), so a large file legitimately needs more than a few
		// seconds to arrive.
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      0, // large/long Range responses must not be cut off
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("listening on %s", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}
