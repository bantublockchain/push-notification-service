package fcm

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	firebase "firebase.google.com/go"
	"firebase.google.com/go/messaging"
	"github.com/bantublockchain/push-notification-service/internal/services"
)

// FCM ...
type FCM struct {
	apiKey string
	log    *log.Logger
	// firebaseApp      *firebase.App
	SenderErrorCount uint
}

// NewFCM ...
func NewFCM(apiKey string, log *log.Logger) (fcm *FCM, err error) {
	// var app *firebase.App
	// if os.Getenv("FCM_LEGACY") != "1" {
	// 	app, err = firebase.NewApp(context.Background(), nil)
	// 	if err != nil {
	// 		log.Fatalf("error initializing FCM app: %v\n", err)
	// 	}
	// }

	fcm = &FCM{
		apiKey: apiKey,
		log:    log,
		// firebaseApp: app,
	}
	return
}

func (fcm *FCM) Logger() *log.Logger {
	return fcm.log
}

// ID ...
func (fcm *FCM) ID() string {
	return "fcm"
}

// String ...
func (fcm *FCM) String() string {
	return "FCM"
}

func (fcm *FCM) NewClient() (services.PumpClient, error) {
	client := &http.Client{
		Timeout: time.Duration(15 * time.Second),
		Transport: &http.Transport{
			MaxIdleConns:    5,
			IdleConnTimeout: 30 * time.Second,
		},
	}
	return client, nil
}

type fcmResponse struct {
	Success int `json:"success"`
	Failure int `json:"failure"`
	Results []struct {
		MessageID      string `json:"message_id"`
		RegistrationID string `json:"registration_id"`
		Error          string `json:"error"`
	} `json:"results"`
}

func (fcm *FCM) SquashAndPushMessage(services.PumpClient, []services.ServiceMessage, services.FeedbackCollector) services.PushStatus {
	panic("not implemented")
}

// POST https://fcm.googleapis.com/v1/projects/myproject-b5ae1/messages:send HTTP/1.1

// Content-Type: application/json
// Authorization: Bearer ya29.ElqKBGN2Ri_Uz...HnS_uNreA

// {
//    "message":{
//       "token":"bk3RNwTe3H0:CI2k_HHwgIpoDKCIZvvDMExUdFQ3P1...",
//       "notification":{
//         "body":"This is an FCM notification message!",
//         "title":"FCM Message"
//       }
//    }
// }

func (fcm *FCM) PushMessage(pclient services.PumpClient, smsg services.ServiceMessage, fc services.FeedbackCollector) services.PushStatus {
	msg := smsg.(fcmMessage)
	startedAt := time.Now()
	var success bool
	var req *http.Request
	var err error
	if len(msg.rawData) < 200 {
		return services.PushStatusSuccess
	}
	if os.Getenv("FCM_LEGACY") == "1" {

		req, err = http.NewRequest("POST", "https://fcm.googleapis.com/fcm/send", bytes.NewBuffer(msg.rawData))
		if err != nil {
			fcm.log.Println("[ERROR] Creating request:", err)
			return services.PushStatusHardFail
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "key="+fcm.apiKey)
		client := pclient.(*http.Client)
		resp, err := client.Do(req)
		if err != nil {
			fcm.log.Println("[ERROR] Posting:", err)
			return services.PushStatusTempFail
		}
		duration := time.Since(startedAt)

		defer func() {
			fc.CountPush(fcm.ID(), success, duration)
		}()

		defer resp.Body.Close()
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			fcm.log.Println("[ERROR] Rejected, status code:", resp.StatusCode)
			return services.PushStatusHardFail
		}
		if resp.StatusCode >= 500 && resp.StatusCode < 600 {
			fcm.log.Println("[ERROR] Upstream error, status code:", resp.StatusCode)
			return services.PushStatusTempFail
		}

		var fr fcmResponse
		err = json.NewDecoder(resp.Body).Decode(&fr)
		if err != nil {
			fcm.log.Println("[ERROR] Decoding response:", err)
			return services.PushStatusTempFail
		}
		regIDs := msg.RegistrationIDs
		if len(regIDs) == 0 {
			regIDs = append(regIDs, msg.To)
		}
		fcm.log.Println("Pushed, took", duration)
		for i, fb := range fr.Results {
			switch fb.Error {
			case "":
				// Noop
			case "InvalidRegistration":
				fallthrough
			case "NotRegistered":
				// you should remove the registration ID from your
				// server database because the application was
				// uninstalled from the device or it does not have a
				// broadcast receiver configured to receive
				// com.google.android.c2dm.intent.RECEIVE intents.
				fc.TokenInvalid(fcm.ID(), regIDs[i])
			case "Unavailable":
				// If it is Unavailable, you could retry to send it in
				// another request.
				fallthrough
			default:
				fcm.log.Println("[ERROR] Sending:", fb.Error)
			}
		}
		success = true
	} else {
		//use modern
		log.Print("NON-LEGACY...")
		var app *firebase.App
		if os.Getenv("FCM_LEGACY") != "1" {
			app, err = firebase.NewApp(context.Background(), nil)
			if err != nil {
				log.Fatalf("error initializing firebase app: %v\n", err)
			}
		}
		m, err := app.Messaging(context.Background())
		if err != nil {
			log.Fatalf("error initializing Messaging: %v\n", err)
		}
		var message *messaging.Message
		if err := json.Unmarshal(msg.rawData, &message); err != nil {
			log.Fatalf("error unmarshalling into firebase message: %v\n", err)
		}
		mid, err := m.Send(context.Background(), message)

		if err != nil {
			fcm.log.Println("[ERROR] send FCM:", err)
			if strings.Contains(strings.ToLower(err.Error()), "sender") {

				if fcm.SenderErrorCount > 5 {
					log.Fatalln("[ERROR] sender error sending FCM:", err)

				}
				fcm.SenderErrorCount++
				return services.PushStatusTempFail

			} else {
				return services.PushStatusTempFail

			}
		}
		duration := time.Since(startedAt)

		defer func() {
			fc.CountPush(fcm.ID(), success, duration)
		}()
		fcm.log.Println("Pushed, took", duration, ", resp: ", mid)
		// log.Println("Pushed, took", duration, ", resp: ", mid)

	}

	return services.PushStatusSuccess
}
