package chore

import (
	"context"
	"time"

	chRepo "donetick.com/core/internal/chore/repo"
	"donetick.com/core/logging"
)

type DeadlineExpiry struct {
	choreRepo *chRepo.ChoreRepository
	stopChan  chan bool
}

func NewDeadlineExpiry(cr *chRepo.ChoreRepository) *DeadlineExpiry {
	return &DeadlineExpiry{
		choreRepo: cr,
		stopChan:  make(chan bool),
	}
}

func (d *DeadlineExpiry) Start(c context.Context) {
	go d.run(c)
}

func (d *DeadlineExpiry) Stop() {
	d.stopChan <- true
}

func (d *DeadlineExpiry) run(c context.Context) {
	log := logging.FromContext(c)
	for {
		select {
		case <-d.stopChan:
			return
		default:
			if err := d.expireDeadlinedChores(c); err != nil {
				log.Error("Error running deadline expiry job", "error", err)
			}
		}
		time.Sleep(5 * time.Minute)
	}
}

func (d *DeadlineExpiry) expireDeadlinedChores(c context.Context) error {
	log := logging.FromContext(c)

	expired, err := d.choreRepo.GetExpiredChores(c)
	if err != nil {
		return err
	}
	log.Debug("Deadline expiry job", "expiredCount", len(expired))

	now := time.Now().UTC()
	for _, chore := range expired {
		var nextDueDate *time.Time

		if chore.IsRecurring() {
			// Advance from the original NextDueDate (preserves schedule rhythm).
			// If the next occurrence is still in the past (chore was missed for multiple
			// periods), keep advancing until we reach a future date.
			nextDueDate, err = scheduleNextDueDate(c, chore, *chore.NextDueDate)
			if err != nil {
				log.Error("Error computing next due date for expired chore", "choreID", chore.ID, "error", err)
				continue
			}
			// Advance past any additional missed periods.
			for nextDueDate != nil && nextDueDate.Before(now) {
				tmp := *chore
				tmp.NextDueDate = nextDueDate
				nextDueDate, err = scheduleNextDueDate(c, &tmp, *nextDueDate)
				if err != nil {
					log.Error("Error advancing next due date", "choreID", chore.ID, "error", err)
					nextDueDate = nil
					break
				}
			}
		}
		// nextDueDate == nil for one-time chores → ExpireChore sets is_active=false.

		if err := d.choreRepo.ExpireChore(c, chore, nextDueDate); err != nil {
			log.Error("Error expiring chore", "choreID", chore.ID, "error", err)
			continue
		}
		log.Debug("Chore expired", "choreID", chore.ID, "isRecurring", chore.IsRecurring(), "nextDueDate", nextDueDate)
	}
	return nil
}
