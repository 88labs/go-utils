package sql_escape

import "testing"

func TestEscapeLike(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "basic",
			args: args{
				s: "%_\\t",
			},
			want: "\\%\\_\\\\t",
		},
		{
			name: "basic2",
			args: args{
				s: "%",
			},
			want: "\\%",
		},
		{
			name: "no-escape",
			args: args{
				s: "t",
			},
			want: "t",
		},
		{
			name: "empty",
			args: args{
				s: "",
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EscapeLike(tt.args.s); got != tt.want {
				t.Errorf("EscapeLike() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEscapeLikeWithChar(t *testing.T) {
	type args struct {
		s string
		c rune
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "basic",
			args: args{
				s: "%_!\\t",
				c: '!',
			},
			want: "!%!_!!\\t",
		},
		{
			name: "basic2",
			args: args{
				s: "%",
				c: '!',
			},
			want: "!%",
		},
		{
			name: "no-escape",
			args: args{
				s: "t",
				c: '!',
			},
			want: "t",
		},
		{
			name: "empty",
			args: args{
				s: "",
				c: '!',
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EscapeLikeWithChar(tt.args.s, tt.args.c); got != tt.want {
				t.Errorf("EscapeLikeWithChar() = %v, want %v", got, tt.want)
			}
		})
	}
}
