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

type AccountBalanceResult struct {
	AccountID                       string  `json:"account_id"`
	ActualBalanceInDB               float64 `json:"actual_balance_in_db"`
	TotalSuccessfulPaymentsRecorded int64   `json:"total_successful_payments_recorded"`
	TotalSumAmountRecorded          float64 `json:"total_sum_amount_recorded"`
}

type CreatePaymentQP syncq.Producer[CreatePaymentReq, struct{}]

func CreatePaymentHandler(app *fiber.App, db *gorm.DB, producer CreatePaymentQP) {
	app.Post("/api/pay", func(c fiber.Ctx) error {
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
		nowTime := time.Now()
		err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&PaymentGorm{
				ID: uuid.NewString(),
				AccountID: req.AccountID,
				Amount: req.Amount,
				CreatedAt: nowTime,
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

func FindAccBalanceHandler(app *fiber.App, db *gorm.DB) {
	app.Get("/api/acc/:id", func(c fiber.Ctx) error {
		accID := c.Params("id")

		var result AccountBalanceResult

		err := db.WithContext(c.Context()).Raw(`
			SELECT
				a.id AS account_id,
				a.balance AS actual_balance_in_db,
				COUNT(p.id) AS total_successful_payments_recorded,
				COALESCE(SUM(p.amount), 0) AS total_sum_amount_recorded
			FROM account_gorms a
			LEFT JOIN payment_gorms p ON p.account_id::uuid = a.id
			WHERE a.id = ?
			GROUP BY a.id, a.balance;
		`, accID).Scan(&result).Error

		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}

		if result.AccountID == "" {
			return fiber.NewError(fiber.StatusNotFound, "account not found")
		}

		return c.JSON(result)
	})
}
