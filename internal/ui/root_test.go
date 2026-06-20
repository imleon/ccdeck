package ui

import "testing"

func TestViewerTitle(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "empty", path: "", want: "(none)"},
		{name: "file", path: "/tmp/root.go", want: "root.go"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := viewerTitle(tt.path); got != tt.want {
				t.Fatalf("viewerTitle(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
