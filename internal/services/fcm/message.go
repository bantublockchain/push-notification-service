package fcm

import (
	"encoding/json"
	"errors"
	"os"

	"github.com/bantublockchain/push-notification-service/internal/services"
)

type fcmMessage struct {
	To              string   `json:"to"`
	RegistrationIDs []string `json:"registration_ids"`
	Token           string   `json:"token"`
	rawData         []byte
}

func (fcmMessage) GetSquashKey() string {
	panic("not implemented")
}

func (fcm *FCM) ConvertMessage(data []byte) (smsg services.ServiceMessage, err error) {
	if len(data) < 100 {
		return nil, errors.New("message to convert has no content")
	}
	var msg fcmMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	if os.Getenv("FCM_LEGACY") == "1" {

		if len(msg.RegistrationIDs) >= 1000 {
			return nil, errors.New("too many tokens")
		}
		if msg.To == "" && len(msg.RegistrationIDs) == 0 {
			return nil, errors.New("no token specified")
		}
		if msg.To != "" && len(msg.RegistrationIDs) > 0 {
			return nil, errors.New("both to/registration_ids specified")
		}
	} else {
		if msg.Token == "" {
			return nil, errors.New("no token specified")
		}
	}

	msg.rawData = data
	return msg, nil
}

// Validate ...
func (fcm *FCM) Validate(data []byte) error {
	_, err := fcm.ConvertMessage(data)
	return err
}
