package routers

import "time"

const minuteKeyLayout = "2006-01-02-15-04"

func minuteKey(t time.Time) string {
	return t.UTC().Format(minuteKeyLayout)
}

func formatBucketKey(t time.Time, format string) string {
	return t.UTC().Format(format)
}
