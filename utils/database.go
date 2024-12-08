package utils

import (
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"log"
	"os"
	"time"

	"git.sr.ht/~aondrejcak/ts-api/models"
	"github.com/uptrace/opentelemetry-go-extra/otelgorm"
	"go.opentelemetry.io/otel"
)

func PrepareDatabase(c *AppConfig) error {
	dbLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold: time.Second,
			LogLevel:      logger.Info,
			Colorful:      true,
		},
	)

	db, err := gorm.Open(mysql.Open(c.DatabaseDSN), &gorm.Config{Logger: dbLogger})
	if err != nil {
		return err
	}

	if err = db.Use(otelgorm.NewPlugin(
		otelgorm.WithAttributes(),
		otelgorm.WithTracerProvider(otel.GetTracerProvider()),
	)); err != nil {
		return err
	}

	db.AutoMigrate(&models.Token{})
	db.AutoMigrate(&models.Company{})
	db.AutoMigrate(&models.Policy{})
	db.AutoMigrate(&models.Role{})
	db.AutoMigrate(&models.Admin{})

	c.DatabaseClient = db

	return nil
}
