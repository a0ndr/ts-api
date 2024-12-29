package kernel

import (
	"errors"
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

func (art *AppRuntime) PrepareDatabase() error {
	dbLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold: time.Second,
			LogLevel:      logger.Info,
			Colorful:      true,
		},
	)

	db, err := gorm.Open(mysql.Open(art.DatabaseDSN), &gorm.Config{Logger: dbLogger})
	if err != nil {
		return err
	}

	if err = db.Use(otelgorm.NewPlugin(
		otelgorm.WithAttributes(),
		otelgorm.WithTracerProvider(otel.GetTracerProvider()),
	)); err != nil {
		return err
	}

	_ = db.AutoMigrate(&models.Token{})
	_ = db.AutoMigrate(&models.Company{})
	//db.AutoMigrate(&models.Policy{})
	//db.AutoMigrate(&models.Role{})
	//db.AutoMigrate(&models.Admin{})
	_ = db.AutoMigrate(&models.Payment{})

	art.DatabaseClient = db

	return nil
}

func (rt *RequestRuntime) First(obj interface{}, where string, args ...interface{}) (bool, error) {
	if err := rt.DB.Where(where, args...).First(obj).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, rt.MakeError(err)
	}
	return true, nil
}
