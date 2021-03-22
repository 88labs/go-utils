package osext

import (
	"os"
	"testing"
)

func TestLookupEnv(t *testing.T) {
	_ = os.Setenv("DEFINED_ENV_VAR", "test")

	type args struct {
		key          string
		defaultValue string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "If environment variable isn't defined, return default value.",
			args: args{
				"UNDEFINED_ENV_VAR",
				"defaultValue",
			},
			want: "defaultValue",
		},
		{
			name: "If environment variable is defined, return its value.",
			args: args{
				"DEFINED_ENV_VAR",
				"defaultValue",
			},
			want: "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LookupEnv(tt.args.key, tt.args.defaultValue); got != tt.want {
				t.Errorf("LookupEnv() = %v, want %v", got, tt.want)
			}
		})
	}
}
