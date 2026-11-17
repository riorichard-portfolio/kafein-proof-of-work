package main

import (
	"log"

	"github.com/gofiber/fiber/v3"

	"github.com/riorichard-portfolio/kafein/syncq"
	"kafein-proof-of-work/db/config/envconfig"
	"kafein-proof-of-work/db/gormdb"
	"kafein-proof-of-work/db/validator"
	"kafein-proof-of-work/flushers"
	"kafein-proof-of-work/handlers"
)

func main() {
	val, err := validator.New()
	if err != nil {
		log.Fatalf("%+v", err)
	}

	appCfg, err := envconfig.NewConfig(".env", val)
	if err != nil {
		log.Fatalf("%+v", err)
	}

	fiberApp := fiber.New()

	gormDB, err := gormdb.NewDB(appCfg.PostgreConfig)
	if err != nil {
		log.Fatalf("%+v", err)
	}

	paymentF := flushers.NewPaymentF(gormDB)
	w, p := syncq.New[handlers.CreatePaymentReq, struct{}](syncq.Config{
		WorkerBatchSize: 300,

		// WorkerBatchSize: 5,
		// QueueSize: 5, // for backpreassure testing (load shedding)
	})
	handlers.CreatePaymentHandler(fiberApp, gormDB, p)
	handlers.FindAccBalanceHandler(fiberApp, gormDB)
	w.Run(paymentF.CreatePayment)
	log.Fatal(fiberApp.Listen(":3000"))
}
