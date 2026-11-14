package flushers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/riorichard-portfolio/kafein/syncq"
	"gorm.io/gorm"

	h "kafein-proof-of-work/handlers"
)

type PaymentFlusher struct {
	db *gorm.DB
}

func NewPaymentF(db *gorm.DB) *PaymentFlusher {
	return &PaymentFlusher{
		db: db,
	}
}

func (pf *PaymentFlusher) bulkAddBalance(bmap map[string]int, tx *gorm.DB) error {
	if len(bmap) == 0 {
		return nil
	}

	var valueRows []string
	var args []interface{}

	for accID, delta := range bmap {
		valueRows = append(valueRows, "(?::uuid, ?::int)")
		args = append(args, accID, delta)
	}

	query := fmt.Sprintf(`
		UPDATE account_gorms AS a 
		SET balance = a.balance + v.delta 
		FROM (VALUES %s) AS v(id, delta) 
		WHERE a.id = v.id
	`, strings.Join(valueRows, ", "))

	return tx.Exec(query, args...).Error
}

func (pf *PaymentFlusher) CreatePayment(ctx context.Context, tasks []syncq.Task[h.CreatePaymentReq, struct{}]) error {
	payments := make([]h.PaymentGorm, 0, len(tasks))
	accIdMap := make(map[string]struct{}, len(tasks))
	nowTime := time.Now()
	for i := range tasks {
		input := tasks[i].GetInput()
		payments = append(payments, h.PaymentGorm{
			ID:        uuid.NewString(),
			AccountID: input.AccountID,
			Amount:    input.Amount,
			CreatedAt: nowTime.UTC(),
		})
		accIdMap[input.AccountID] = struct{}{}
	}
	// validate accounts
	accIds := make([]string, 0, len(tasks))
	for accId := range accIdMap {
		accIds = append(accIds, accId)
	}
	var accounts []h.AccountGorm
	err := pf.db.Where("id IN ?", accIds).Find(&accounts).Error
	if err != nil {
		for i := range tasks {
			tasks[i].SendError(err)
		}
		return err
	}
	validAccIDMap := make(map[string]struct{}, len(tasks))
	for _, acc := range accounts {
		validAccIDMap[acc.ID] = struct{}{}
	}
	validIndices := make([]int, 0, len(tasks))
	validIdx := 0
	accBalMap := make(map[string]int, len(tasks)) // map of account id and sum of amount to be add to balance
	for i, p := range payments {
		_, ok := validAccIDMap[p.AccountID]
		if ok {
			validIndices = append(validIndices, i)
			payments[validIdx] = p
			accBalMap[p.AccountID] += p.Amount
			validIdx++
		} else {
			tasks[i].SendError(errors.New("account not found"))
		}
	}
	if validIdx == 0 {
		return nil
	}
	payments = payments[:validIdx]
	err = pf.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err = tx.Create(payments).Error
		if err != nil {
			return err
		}
		err := pf.bulkAddBalance(accBalMap, tx)
		return err
	})
	for _, validI := range validIndices {
		tasks[validI].SendError(err)
	}
	return err
}
