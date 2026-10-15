package handlers

import (
	"strconv"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func ptrFuture() *time.Time {
	t := time.Now().AddDate(1, 0, 0)
	return &t
}

func tsFuture() *timestamppb.Timestamp {
	return timestamppb.New(time.Now().AddDate(1, 0, 0))
}

func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}
