package initializer

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/bignyap/go-admin/internal/caching"
	"github.com/bignyap/go-admin/internal/database/sqlcgen"
	cachemanagement "github.com/bignyap/go-admin/internal/gatekeeper/service/CacheManagement"
	"github.com/bignyap/go-admin/internal/initialize"
	"github.com/bignyap/go-admin/internal/router"
	"github.com/bignyap/go-utilities/logger/api"
	"github.com/bignyap/go-utilities/logger/config"
	"github.com/bignyap/go-utilities/logger/factory"
	"github.com/bignyap/go-utilities/server"
	"github.com/go-playground/validator"
	"github.com/jackc/pgx/v5/pgxpool"
)

type GateKeeperService struct {
	Logger         api.Logger
	ResponseWriter *server.ResponseWriter
	DB             *sqlcgen.Queries
	Conn           *pgxpool.Pool
	Validator      *validator.Validate
	CacheContoller *caching.CacheController
	CacheManager   *cachemanagement.CacheManagementService
	Mode           string
	Target         string
	stopFlush      chan struct{}
}

func NewGateKeeperService(
	logger api.Logger,
	conn *pgxpool.Pool,
	validator *validator.Validate,
	cacheController *caching.CacheController,
	mode string,
	target string,
	cacheManager *cachemanagement.CacheManagementService,
) *GateKeeperService {
	return &GateKeeperService{
		Logger:         logger,
		Validator:      validator,
		DB:             sqlcgen.New(conn),
		Conn:           conn,
		CacheContoller: cacheController,
		CacheManager:   cacheManager,
		Mode:           mode,
		Target:         target,
		stopFlush:      make(chan struct{}),
	}
}

func (s *GateKeeperService) Setup(server server.Server) error {
	setupLogger := s.Logger.WithComponent("server.Setup")
	setupLogger.Info("Starting")

	s.ResponseWriter = server.GetResponseWriter()

	router.RegisterGateKeeperHandlers(
		server.Router(),
		s.Logger,
		s.ResponseWriter,
		s.DB,
		s.Conn,
		s.Validator,
		s.CacheContoller,
		s.Mode,
		s.Target,
	)

	// Start periodic cache sync
	cachemanagement.StartPeriodicFlush(s.CacheManager, 30*time.Second, s.stopFlush)

	setupLogger.Info("Completed")
	return nil
}

func (s *GateKeeperService) Shutdown() error {

	shtLogger := s.Logger.WithComponent("server.Shutdown")
	shtLogger.Info("Starting")

	// Stop periodic flushing
	close(s.stopFlush)

	ctx := context.Background()

	// Flush caches to Redis/DB
	s.CacheManager.SyncIncrementalToRedis(ctx, "usage")
	s.CacheManager.SyncAggregatedToDB(ctx, "usage", func(key string, count int) error {
		return s.CacheManager.IncrementUsageFromCacheKey(ctx, key, count)
	})
	shtLogger.Info("Cache flushed")

	// Close Redis connection if possible
	if err := s.CacheContoller.Close(); err != nil {
		shtLogger.Error("Error closing Redis", err)
	} else {
		shtLogger.Info("Redis connection closed")
	}

	// Close DB
	if s.Conn != nil {
		s.Conn.Close()
		shtLogger.Info("Database connection pool closed")
	}

	shtLogger.Info("Completed")
	return nil
}

func InitializeGateKeeperServer() {
	if err := initialize.GetEnvVals(); err != nil {
		log.Fatalf("Failed to load environment variables: %v", err)
	}

	environment := os.Getenv("ENVIRONMENT")
	if environment == "" {
		environment = "dev"
	}
	mode := os.Getenv("GATEKEEPER_MODE") // "proxy", "middleware", or "auth-middleware"
	target := os.Getenv("PROXY_TARGET")  // required if mode is "proxy"

	if mode == "proxy" && target == "" {
		log.Fatal("PROXY_TARGET must be set in proxy mode")
	}

	// Logger setup
	var logConfig config.LogConfig
	if environment == "prod" {
		logConfig = config.ProductionConfig()
	} else {
		logConfig = config.DevelopmentConfig()
	}
	logger, _ := factory.NewLogger(logConfig)

	// Database connection
	conn, err := initialize.LoadDBConn()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer conn.Close()

	// Validator
	validator := validator.New()

	// Redis cache controller from env
	cacheController, err := initialize.LoadRedisController()
	if err != nil {
		log.Fatalf("Failed to initialize Redis: %v", err)
	}

	// Cache manager
	cacheManager := &cachemanagement.CacheManagementService{
		Logger:    logger,
		Cache:     cacheController,
		DB:        sqlcgen.New(conn),
		Conn:      conn,
		Validator: validator,
	}

	// Main service
	gkService := NewGateKeeperService(
		logger, conn, validator, cacheController,
		mode, target, cacheManager,
	)

	// Start web server
	if err := initialize.InitializeWebServer(logger, gkService); err != nil {
		log.Fatalf("Failed to start web server: %v", err)
	}
}
