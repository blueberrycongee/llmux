package routers

import (
	"strconv"
	"time"
)

func minuteKey(t time.Time) string {
	return strconv.FormatInt(t.UTC().Unix()/60, 10)
}
