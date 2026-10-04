package internal

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestFileContentBase64(t *testing.T) {
	tests := []struct {
		name    string
		content []byte
		want    types.String
	}{
		{
			name:    "not a regular file",
			content: nil,
			want:    types.StringNull(),
		},
		{
			name:    "empty file",
			content: []byte{},
			want:    types.StringValue(""),
		},
		{
			name:    "text",
			content: []byte("héllo\n"),
			want:    types.StringValue("aMOpbGxvCg=="),
		},
		{
			name:    "binary",
			content: []byte{0xff, 0xfe, 0x00, 0x01},
			want:    types.StringValue("//4AAQ=="),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fileContentBase64(&FileInfo{Content: tt.content})
			if !got.Equal(tt.want) {
				t.Errorf("content_base64 = %s, want %s", got, tt.want)
			}
		})
	}
}
