package shoutrrr

import (
	"context"
	"errors"
	"strings"

	"donetick.com/core/config"
	nModel "donetick.com/core/internal/notifier/model"
	"donetick.com/core/logging"
	"github.com/nicholas-fedor/shoutrrr"
	shoutrrrTypes "github.com/nicholas-fedor/shoutrrr/pkg/types"
)

type router interface {
	Send(message string, params *shoutrrrTypes.Params) []error
}

// ShoutrrrNotifier manages Shoutrrr notifications.
type ShoutrrrNotifier struct {
	shoutrrrUrl string
	router      router
}

func NewShoutrrrNotifier(cfg *config.Config) (*ShoutrrrNotifier, error) {
	sender, err := shoutrrr.CreateSender(cfg.Shoutrrr.ShoutrrrUrl)
	if err != nil {
		return nil, err
	}

	s := &ShoutrrrNotifier{
		shoutrrrUrl: cfg.Shoutrrr.ShoutrrrUrl,
		router:      sender,
	}

	return s, nil
}

func (s *ShoutrrrNotifier) SendNotification(ctx context.Context, notification *nModel.NotificationDetails) error {
	log := logging.FromContext(ctx)

	params := &shoutrrrTypes.Params{}
	errs := s.router.Send(notification.Text, params)

	if len(errs) > 0 {
		var errstrings []string

		for _, err := range errs {
			if err == nil || err.Error() == "" {
				continue
			}
			errstrings = append(errstrings, err.Error())
		}
		//sometimes there are empty errs, we're going to skip them.
		if len(errstrings) == 0 {
			return nil
		} else {
			log.Errorf("One or more errors occurred while sending notifications for %s:", s.shoutrrrUrl)
			log.Error(errs)

			return errors.New(strings.Join(errstrings, "\n"))
		}
	}

	return nil
}
