package tspb_cast

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// ToTime cast Timestamp to Time.
//
//goland:noinspection GoUnusedExportedFunction
func ToTime(from *timestamppb.Timestamp) time.Time {
	if from == nil {
		return time.Time{}
	} else {
		return from.AsTime()
	}
}

// ToTimestamp cast Time to Timestamp.
//
//goland:noinspection GoUnusedExportedFunction
func ToTimestamp(from time.Time) *timestamppb.Timestamp {
	if from.IsZero() {
		return nil
	} else {
		return timestamppb.New(from)
	}
}
