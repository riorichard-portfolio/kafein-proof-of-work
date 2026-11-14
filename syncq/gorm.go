package syncq

import "time"

type PaymentGorm struct {
	ID        string    `gorm:"type:uuid;column:id;primaryKey"`
	AccountID string    `gorm:"type:varchar(255);column:account_id;not null"`
	Amount    int       `gorm:"type:int;column:amount;not null"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
}

type AccountGorm struct {
	ID      string `gorm:"type:uuid;column:id;primaryKey"`
	Name    string `gorm:"type:varchar(255);column:name;not null"`
	Balance int    `gorm:"type:int;column:balance;not null"`
}
