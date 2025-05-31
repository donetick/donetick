package utils

import "time"

func IsValidTimezone(tz string) bool {
	_, err := time.LoadLocation(tz)
	return err == nil
}
