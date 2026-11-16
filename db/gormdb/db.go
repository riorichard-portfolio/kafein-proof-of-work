package gormdb

import (
	"time"

	"kafein-proof-of-work/db/config"
	h "kafein-proof-of-work/handlers"

	"github.com/cockroachdb/errors"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewDB(
	cfg config.PostgreConfig,
) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.URL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to open gorm connection")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get underlying sql.DB")
	}

	sqlDB.SetMaxIdleConns(10)                  // Jumlah koneksi idle di pool
	sqlDB.SetMaxOpenConns(100)                 // Batas maksimal koneksi simultan ke Postgres
	sqlDB.SetConnMaxLifetime(1 * time.Hour)    // Umur maksimal satu koneksi sebelum didaur ulang
	sqlDB.SetConnMaxIdleTime(15 * time.Minute) // Durasi koneksi idle ditutup dari pool

	err = sqlDB.Ping()
	if err != nil {
		return nil, errors.Wrap(err, "failed to ping DB")
	}

	err = db.AutoMigrate(&h.AccountGorm{}, &h.PaymentGorm{})
	if err != nil {
		return nil, errors.Wrap(err,"failed to auto migrate db")
	}
	err = seedAccounts(db)
	if err != nil {
		return nil, errors.Wrap(err,"failed to seed db")
	}

	return db, nil
}

func seedAccounts(db *gorm.DB) error {
	var count int64
	db.Model(&h.AccountGorm{}).Count(&count)
	if count > 0 {
		return nil
	}

	hotAccountID := "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11" // UUID static

	hotAccount := h.AccountGorm{
		ID:      hotAccountID,
		Name:    "Merchant Hot-Row Utama",
		Balance: 0, // init 0 balance
	}

	if err := db.Create(&hotAccount).Error; err != nil {
		return errors.Wrap(err, "failed to seed account")
	}

	return nil
}
