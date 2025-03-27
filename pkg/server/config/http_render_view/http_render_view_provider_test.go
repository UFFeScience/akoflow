package http_render_view

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDict(t *testing.T) {
	tests := []struct {
		name    string
		input   []interface{}
		want    map[string]interface{}
		wantErr bool
	}{
		{
			name:    "even number of arguments",
			input:   []interface{}{"key1", "value1", "key2", "value2"},
			want:    map[string]interface{}{"key1": "value1", "key2": "value2"},
			wantErr: false,
		},
		{
			name:    "odd number of arguments",
			input:   []interface{}{"key1", "value1", "key2"},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "non-string key",
			input:   []interface{}{1, "value1", "key2", "value2"},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := dict(tt.input...)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
