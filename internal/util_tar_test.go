package internal

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestFileContentValues(t *testing.T) {
	tests := []struct {
		name              string
		content           []byte
		wantContent       types.String
		wantContentBase64 types.String
	}{
		{
			name:              "not a regular file",
			content:           nil,
			wantContent:       types.StringNull(),
			wantContentBase64: types.StringNull(),
		},
		{
			name:              "empty file",
			content:           []byte{},
			wantContent:       types.StringValue(""),
			wantContentBase64: types.StringValue(""),
		},
		{
			name:              "text",
			content:           []byte("héllo\n"),
			wantContent:       types.StringValue("héllo\n"),
			wantContentBase64: types.StringValue("aMOpbGxvCg=="),
		},
		{
			name:              "binary",
			content:           []byte{0xff, 0xfe, 0x00, 0x01},
			wantContent:       types.StringNull(),
			wantContentBase64: types.StringValue("//4AAQ=="),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, contentBase64 := fileContentValues(&FileInfo{Content: tt.content})
			if !content.Equal(tt.wantContent) {
				t.Errorf("content = %s, want %s", content, tt.wantContent)
			}
			if !contentBase64.Equal(tt.wantContentBase64) {
				t.Errorf("content_base64 = %s, want %s", contentBase64, tt.wantContentBase64)
			}
		})
	}
}
