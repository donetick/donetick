package utils

import "time"

func IsValidTimezone(tz string) bool {
	_, err := time.LoadLocation(tz) //TODO: Log invalid timezone?
	return err == nil
}
