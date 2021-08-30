package tspb_cast

import (
	"reflect"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestToTime(t *testing.T) {
	now := time.Now().UTC()
	type args struct {
		from *timestamppb.Timestamp
	}
	tests := []struct {
		name string
		args args
		want time.Time
	}{
		{
			name: "cast nil to Time",
			args: args{
				from: nil,
			},
			want: time.Time{},
		},
		{
			name: "cast Timestamp to Time",
			args: args{
				from: timestamppb.New(now),
			},
			want: now,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToTime(tt.args.from); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ToTime() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToTimestamp(t *testing.T) {
	now := time.Now().UTC()

	type args struct {
		from time.Time
	}
	tests := []struct {
		name string
		args args
		want *timestamppb.Timestamp
	}{
		{
			name: "cast zero value of Time to Timestamp",
			args: args{
				from: time.Time{},
			},
			want: nil,
		},
		{
			name: "cast Time to Timestamp",
			args: args{
				from: now,
			},
			want: timestamppb.New(now),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToTimestamp(tt.args.from); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ToTimestamp() = %v, want %v", got, tt.want)
			}
		})
	}
}
