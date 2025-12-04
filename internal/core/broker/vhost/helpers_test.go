package vhost

import "testing"

func Test_parseMaxPriorityArgument(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		want uint8
		ok   bool
	}{
		{
			name: "valid int",
			args: map[string]interface{}{"x-max-priority": int(10)},
			want: 10,
			ok:   true,
		},
		{
			name: "valid int32",
			args: map[string]interface{}{"x-max-priority": int32(20)},
			want: 20,
			ok:   true,
		},
		{
			name: "valid int64",
			args: map[string]interface{}{"x-max-priority": int64(30)},
			want: 30,
			ok:   true,
		},
		{
			name: "valid float64",
			args: map[string]interface{}{"x-max-priority": float64(40)},
			want: 40,
			ok:   true,
		},
		{
			name: "invalid type",
			args: map[string]interface{}{"x-max-priority": "invalid"},
			want: 0,
			ok:   false,
		},
		{
			name: "negative value",
			args: map[string]interface{}{"x-max-priority": int(-5)},
			want: 0,
			ok:   false,
		},
		{
			name: "zero value",
			args: map[string]interface{}{"x-max-priority": int(0)},
			want: 0,
			ok:   false,
		},
		{
			name: "value exceeds 255",
			args: map[string]interface{}{"x-max-priority": int(300)},
			want: 255,
			ok:   true,
		},
		{
			name: "missing argument",
			args: map[string]interface{}{},
			want: 0,
			ok:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseMaxPriorityArgument(tt.args)
			if got != tt.want || ok != tt.ok {
				t.Errorf("parseMaxPriorityArgument() = %v, %v; want %v, %v", got, ok, tt.want, tt.ok)
			}
		})
	}
}
