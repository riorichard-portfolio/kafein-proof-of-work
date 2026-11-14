package handlers

import (
	"context"
	"errors"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/riorichard-portfolio/kafein/syncq"
	"gorm.io/gorm"
)

type CreatePaymentReq struct {
	AccountID string `json:"account_id" validate:"required"`
	Amount    int    `json:"amount" validate:"required"`
}

type CreatePaymentQP syncq.Producer[CreatePaymentReq, struct{}]

func CreatePaymentHandler(app *fiber.App, db *gorm.DB, producer CreatePaymentQP) {
	app.Post("/api/transfer", func(c fiber.Ctx) error {
		var req CreatePaymentReq
		if err := c.Bind().Body(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		ctx, cancel := context.WithTimeout(c.Context(), 3*time.Second)
		defer cancel()
		mode := c.Get("X-Execution-Mode")
		if mode == "syncq" {

			_, err := producer.AddTaskQ(ctx, req)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}
			return c.JSON(fiber.Map{"status": "processed_via_syncq"})
		}

		// Baseline direct GORM transaction
		err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&PaymentGorm{
				ID: uuid.NewString(),
			}).Error; err != nil {
				return err
			}

			txUpdate := tx.Model(&AccountGorm{}).
				Where("id = ?", req.AccountID).
				Update("balance", gorm.Expr("balance + ?", req.Amount))
			if txUpdate.Error != nil {
				return txUpdate.Error
			}

			if txUpdate.RowsAffected == 0 {
				return errors.New("account not found")
			}

			return nil
		})

		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(fiber.Map{"status": "success_baseline"})
	})
}
