package main

import (
	"context"
	"flag"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/redis/go-redis/v9"

	"restaurants/internal/config"
	handlermanager "restaurants/internal/handlers/manager"
	"restaurants/pkg/client/postgresql"
	"restaurants/pkg/fcm"
	"restaurants/pkg/logging"
	"restaurants/pkg/sms_sender"
)

func main() {
	var cfgPath string
	flag.StringVar(&cfgPath, "c", "./etc/railway_internal_config.yaml", "path to config file") //   "./etc/railway_internal_config.yaml"
	flag.Parse()

	cfg := config.GetConfig(cfgPath)
	logger := logging.GetLogger()

	//rds, err := newRedisClient(ctx, cfg.Redis)
	//if err != nil {
	//	panic(fmt.Sprintf("Redis connection error: %v", err))
	//}

	smsSender, err := sms_sender.NewSmsSender(cfg.SmsSender)
	if err != nil {
		panic(err)
	}

	// fcmClient, err := fcm.NewFCMClient("./internal/config/serviceAccountKey.json")
	// if err != nil {
	// 	panic(fmt.Sprintf("FCM init error: %v", err))
	// }

	var fcmClient *fcm.FCMClient = nil

	postgresSQLClient, err := startPostgresql(cfg, logger)
	if err != nil {
		panic(err)
	}
	defer postgresSQLClient.Close()
	//repository := reservationdb.NewRepository(postgresSQLClient, logger)

	// reservationDB := jobs.ReservationDB(postgresSQLClient, fcmClient)
	// go reservationDB.StartReservationCheckerScheduler(context.TODO(), repository)

	// stayedMinutesDB := jobs.StayedMinutesDB(postgresSQLClient, fcmClient)
	// go stayedMinutesDB.StartStayedMinutesScheduler(context.TODO())

	// cleanDB := jobs.CleanDataBase(postgresSQLClient, fcmClient)
	// go cleanDB.StartCleanDataBaseScheduler(context.TODO(), repository)

	start(handlermanager.Manager(postgresSQLClient, logger, smsSender, fcmClient), cfg)
}

func startPostgresql(cfg *config.Config, _ *logging.Logger) (*pgxpool.Pool, error) {
	postgresSQLClient, err := postgresql.NewClient(context.TODO(), 3, cfg.Storage)
	if err != nil {
		return nil, err
	}

	return postgresSQLClient, err
}

func start(router *gin.Engine, cfg *config.Config) {
	logger := logging.GetLogger()
	logger.Info("start application")

	router.StaticFS("/uploads", gin.Dir(cfg.PublicFilePath, false))
	router.Static("/public", "./public")
	err := router.Run("0.0.0.0:8081")
	if err != nil {
		return
	}
}

func newRedisClient(ctx context.Context, cfg config.RedisConfig) (*redis.Client, error) {
	c := redis.NewClient(&redis.Options{
		Addr:            cfg.Addr,
		ClientName:      cfg.ClientName,
		Password:        cfg.Password,
		MaxRetries:      cfg.MaxRetries,
		DialTimeout:     time.Duration(cfg.DailTimeout) * time.Second,
		ReadTimeout:     time.Duration(cfg.ReadTimeout) * time.Second,
		WriteTimeout:    time.Duration(cfg.WriteTimeout) * time.Second,
		ConnMaxIdleTime: time.Duration(cfg.IdleTimeout) * time.Minute,
		ConnMaxLifetime: time.Duration(cfg.MaxConnLifeTime) * time.Minute,
		MinIdleConns:    cfg.MinIdleConn,
	})

	return c, c.Ping(ctx).Err()
}
