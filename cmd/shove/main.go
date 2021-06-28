package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bantublockchain/push-notification-service/internal/queue"
	"github.com/bantublockchain/push-notification-service/internal/queue/memory"
	"github.com/bantublockchain/push-notification-service/internal/queue/redis"
	"github.com/bantublockchain/push-notification-service/internal/server"
	"github.com/bantublockchain/push-notification-service/internal/services"
	"github.com/bantublockchain/push-notification-service/internal/services/apns"
	"github.com/bantublockchain/push-notification-service/internal/services/email"
	"github.com/bantublockchain/push-notification-service/internal/services/fcm"
	"github.com/bantublockchain/push-notification-service/internal/services/telegram"
	"github.com/bantublockchain/push-notification-service/internal/services/webpush"
	"github.com/shopspring/decimal"
)

var apiAddr = flag.String("api-addr", ":8322", "API address to listen to")

var apnsCertificate = flag.String("apns-certificate-path", "", "APNS certificate path")
var apnsSandboxCertificate = flag.String("apns-sandbox-certificate-path", "", "APNS sandbox certificate path")
var apnsWorkers = flag.Int("apns-workers", 4, "The number of workers pushing APNS messages")

// var fcmAPIKey = flag.String("fcm-api-key", "", "FCM API key")

// var fcmWorkers = flag.Int("fcm-workers", 4, "The number of workers pushing FCM messages")
var fcmWorkers = 4

// var redisURL = flag.String("queue-redis", "", "Use Redis queue (Redis URL)")
// var redisPassword = flag.String("queue-redis-password", "", "Use Password for Redis queue (Redis URL)")

var webPushVAPIDPublicKey = flag.String("webpush-vapid-public-key", "", "VAPID public key")
var webPushVAPIDPrivateKey = flag.String("webpush-vapid-private-key", "", "VAPID public key")
var webPushWorkers = flag.Int("webpush-workers", 8, "The number of workers pushing Web messages")

var telegramBotToken = flag.String("telegram-bot-token", "", "Telegram bot token")
var telegramWorkers = flag.Int("telegram-workers", 2, "The number of workers pushing Telegram messages")
var telegramRateAmount = flag.Int("telegram-rate-amount", 0, "Telegram max. rate (amount)")
var telegramRatePer = flag.Int("telegram-rate-per", 0, "Telegram max. rate (per seconds)")

var emailHost = flag.String("email-host", "", "Email host")
var emailPort = flag.Int("email-port", 25, "Email port")
var emailTLS = flag.Bool("email-tls", false, "Use TLS")
var emailTLSInsecure = flag.Bool("email-tls-insecure", false, "Skip TLS verification")
var emailRateAmount = flag.Int("email-rate-amount", 0, "Email max. rate (amount)")
var emailRatePer = flag.Int("email-rate-per", 0, "Email max. rate (per seconds)")

func newServiceLog(prefix string) *log.Logger {
	return log.New(log.Writer(), prefix+": ", log.Flags())
}

func main() {
	log.SetFlags(log.Flags() | log.Lmsgprefix)
	flag.Parse()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	var qf queue.QueueFactory
	if os.Getenv("ENABLE_CACHING") == "1" && len(os.Getenv("REDIS_HOST")) > 0 && len(os.Getenv("REDIS_PORT")) > 0 && len(os.Getenv("REDIS_PASSWORD")) > 0 {
		log.Println("Using Redis queue at", fmt.Sprintf("redis://%v:%v", os.Getenv("REDIS_HOST"), os.Getenv("REDIS_PORT")))
		qf = redis.NewQueueFactory(fmt.Sprintf("redis://%v:%v", os.Getenv("REDIS_HOST"), os.Getenv("REDIS_PORT")), os.Getenv("REDIS_PASSWORD"))
	} else {
		log.Println("Using non-persistent in-memory queue")
		qf = memory.MemoryQueueFactory{}
	}
	s := server.NewServer(*apiAddr, qf)

	if *apnsCertificate != "" {
		apns, err := apns.NewAPNS(*apnsCertificate, true, newServiceLog("apns"))
		if err != nil {
			log.Fatal("[ERROR] Setting up APNS service:", err)
		}
		if err := s.AddService(apns, *apnsWorkers, services.SquashConfig{}); err != nil {
			log.Fatal("[ERROR] Adding APNS service:", err)
		}
	}

	if *apnsSandboxCertificate != "" {
		apns, err := apns.NewAPNS(*apnsSandboxCertificate, false, newServiceLog("apns-sandbox"))
		if err != nil {
			log.Fatal("[ERROR] Setting up APNS sandbox service:", err)
		}
		if err := s.AddService(apns, *apnsWorkers, services.SquashConfig{}); err != nil {
			log.Fatal("[ERROR] Adding APNS sandbox service:", err)
		}
	}

	if os.Getenv("FCM_API_KEY") != "" {
		fcm, err := fcm.NewFCM(os.Getenv("FCM_API_KEY"), newServiceLog("fcm"))
		if err != nil {
			log.Fatal("[ERROR] Setting up FCM service:", err)
		}
		if len(os.Getenv("FCM_WORKERS")) > 0 {
			if decimal.RequireFromString(os.Getenv("FCM_WORKERS")).IsPositive() {
				fcmWorkers = int(decimal.RequireFromString(os.Getenv("FCM_WORKERS")).IntPart())
			}
		}
		if err := s.AddService(fcm, fcmWorkers, services.SquashConfig{}); err != nil {
			log.Fatal("[ERROR] Adding FCM service:", err)
		}
	}

	if *webPushVAPIDPrivateKey != "" {
		web, err := webpush.NewWebPush(*webPushVAPIDPublicKey, *webPushVAPIDPrivateKey, newServiceLog("webpush"))
		if err != nil {
			log.Fatal("[ERROR] Setting up WebPush service:", err)
		}
		if err := s.AddService(web, *webPushWorkers, services.SquashConfig{}); err != nil {
			log.Fatal("[ERROR] Adding WebPush service:", err)
		}
	}

	if *telegramBotToken != "" {
		tg, err := telegram.NewTelegramService(*telegramBotToken, newServiceLog("telegram"))
		if err != nil {
			log.Fatal("[ERROR] Setting up Telegram service:", err)
		}
		if err := s.AddService(tg, *telegramWorkers, services.SquashConfig{
			RateMax: *telegramRateAmount,
			RatePer: time.Second * time.Duration(*telegramRatePer),
		}); err != nil {
			log.Fatal("[ERROR] Adding Telegram service:", err)
		}
	}

	if *emailHost != "" {
		config := email.EmailConfig{
			EmailHost:   *emailHost,
			EmailPort:   *emailPort,
			TLS:         *emailTLS,
			TLSInsecure: *emailTLSInsecure,
			Log:         newServiceLog("email"),
		}
		email, err := email.NewEmailService(config)
		if err != nil {
			log.Fatal("[ERROR] Setting up email service:", err)
		}
		if err := s.AddService(email, 1, services.SquashConfig{
			RateMax: *emailRateAmount,
			RatePer: time.Second * time.Duration(*emailRatePer),
		}); err != nil {
			log.Fatal("[ERROR] Adding email service:", err)
		}
	}

	go func() {
		log.Println("Serving on", *apiAddr)
		err := s.Serve()
		if err != nil {
			log.Fatal("[ERROR] Serving:", err)
		}
	}()
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.Shutdown(ctx)
	log.Println("Exiting")
}
