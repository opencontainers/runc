package utils

import (
	"errors"
	"maps"
	"testing"
)

func TestParseKV(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    map[string]string
		wantErr error
	}{
		{
			name:  "empty string",
			input: "",
			want:  map[string]string{},
		},
		{
			name:  "single unquoted pair",
			input: "key=value",
			want:  map[string]string{"key": "value"},
		},
		{
			name:  "multiple unquoted pairs",
			input: "k1=v1,k2=v2,k3=v3",
			want: map[string]string{
				"k1": "v1",
				"k2": "v2",
				"k3": "v3",
			},
		},
		{
			name:  "double quoted value containing comma",
			input: `context="system_u:object_r:container_file_t:s0:c344,c805"`,
			want: map[string]string{
				"context": "system_u:object_r:container_file_t:s0:c344,c805",
			},
		},
		{
			name:  "single quoted value containing comma",
			input: `key='one,two'`,
			want: map[string]string{
				"key": "one,two",
			},
		},
		{
			name:  "escaped comma",
			input: `lowerdir=dir1\,dir2`,
			want: map[string]string{
				"lowerdir": "dir1,dir2",
			},
		},
		{
			name:  "escaped comma inside double quotes",
			input: `key="a\,b"`,
			want: map[string]string{
				"key": "a,b",
			},
		},
		{
			name:  "key without value",
			input: "k1,k2=v2",
			want: map[string]string{
				"k1": "",
				"k2": "v2",
			},
		},
		{
			name:  "escaped double quote inside double quotes",
			input: `key="one\"two"`,
			want: map[string]string{
				"key": `one"two`,
			},
		},
		{
			name:  "escaped single quote inside single quotes",
			input: `key='one\'two'`,
			want: map[string]string{
				"key": "one'two",
			},
		},
		{
			name:  "escaped single quote without quotes",
			input: `key=one\'two`,
			want: map[string]string{
				"key": "one'two",
			},
		},
		{
			name:  "empty value",
			input: "key=",
			want: map[string]string{
				"key": "",
			},
		},
		{
			name:  "empty quoted value",
			input: `key=""`,
			want: map[string]string{
				"key": "",
			},
		},
		{
			name:  "double quote in the middle of a value",
			input: `key=one"two`,
			want: map[string]string{
				"key": `one"two`,
			},
		},
		{
			name:  "single quote inside double quotes",
			input: `key="one'two"`,
			want: map[string]string{
				"key": "one'two",
			},
		},
		{
			name:  "double quote inside single quotes",
			input: `key='one"two'`,
			want: map[string]string{
				"key": `one"two`,
			},
		},
		{
			name:  "text after closing quote is appended",
			input: `key="a"b`,
			want: map[string]string{
				"key": "ab",
			},
		},
		{
			name:  "multiple equals signs in value",
			input: "ke=y=val",
			want: map[string]string{
				"ke": "y=val",
			},
		},
		{
			name:  "escaped comma in key",
			input: `ke\,y=val`,
			want: map[string]string{
				"ke,y": "val",
			},
		},
		{
			name:  "escaped backslash",
			input: `key=a\\b`,
			want: map[string]string{
				"key": `a\b`,
			},
		},
		{
			name:  "whitespace is preserved",
			input: "key1=v1, key2=v2",
			want: map[string]string{
				"key1":  "v1",
				" key2": "v2",
			},
		},
		{
			name:  "empty pairs are skipped",
			input: ",k=v,,",
			want: map[string]string{
				"k": "v",
			},
		},
		{
			name:  "duplicate keys keep the last value",
			input: "k=a,k=b",
			want: map[string]string{
				"k": "b",
			},
		},
		{
			name:  "multi-byte characters",
			input: "äh=ö,k=ünïcode",
			want: map[string]string{
				"äh": "ö",
				"k":  "ünïcode",
			},
		},
		{
			name:    "unclosed double quote",
			input:   `key="val`,
			wantErr: ErrUnclosedDoubleQuote,
		},
		{
			name:    "unclosed single quote",
			input:   `key='val`,
			wantErr: ErrUnclosedSingleQuote,
		},
		{
			name:    "unfinished escape sequence",
			input:   `key=val\`,
			wantErr: ErrUnfinishedEscape,
		},
		{
			name:  "mixed complex case",
			input: `k1=v1,k2="a,b,c",k3='x,y',k4=ab\,cd,k5,k6=one\\two`,
			want: map[string]string{
				"k1": "v1",
				"k2": "a,b,c",
				"k3": "x,y",
				"k4": "ab,cd",
				"k5": "",
				"k6": `one\two`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseKV(tt.input)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ParseKV() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				if got != nil {
					t.Errorf("ParseKV() returned map %v on error, want nil", got)
				}
				return
			}
			if !maps.Equal(got, tt.want) {
				t.Errorf("ParseKV() got = %v, want %v", got, tt.want)
			}
		})
	}
}
