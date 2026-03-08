package main

import (
	"strings"
	"testing"
)

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		expected    string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid instagram https URL",
			raw:         "https://instagram.com/p/CLxyz",
			expected:    "https://instagram.com/p/CLxyz",
			expectError: false,
		},
		{
			name:        "valid facebook https URL",
			raw:         "https://www.facebook.com/reel/123",
			expected:    "https://www.facebook.com/reel/123",
			expectError: false,
		},
		{
			name:        "valid fb.watch URL",
			raw:         "https://fb.watch/xyz",
			expected:    "https://fb.watch/xyz",
			expectError: false,
		},
		{
			name:        "missing scheme prepends https",
			raw:         "facebook.com/watch?v=123",
			expected:    "https://facebook.com/watch?v=123",
			expectError: false,
		},
		{
			name:        "collapsed https single slash",
			raw:         "https:/facebook.com/watch?v=123",
			expected:    "https://facebook.com/watch?v=123",
			expectError: false,
		},
		{
			name: "strips query parameters (wait, FB needs query for watch?v=)",
			// Actually my implementation currently strips RawQuery and Fragment.
			// Let's re-evaluate that. yt-dlp might need the query for some FB URLs.
			// But the previous implementation for Instagram also stripped it.
			raw:         "https://instagram.com/p/CLxyz?igshid=123&utm_source=wa",
			expected:    "https://instagram.com/p/CLxyz",
			expectError: false,
		},
		{
			name:        "keeps query for facebook",
			raw:         "https://facebook.com/watch?v=123",
			expected:    "https://facebook.com/watch?v=123",
			expectError: false,
		},
		{
			name:        "http scheme is rejected",
			raw:         "http://facebook.com/watch?v=123",
			expected:    "",
			expectError: true,
			errorMsg:    "only HTTPS URLs are accepted",
		},
		{
			name:        "unsupported host tiktok",
			raw:         "https://tiktok.com/v/123",
			expected:    "",
			expectError: true,
			errorMsg:    "not a supported URL",
		},
		{
			name:        "semicolon in path is rejected",
			raw:         "https://instagram.com/p/abc;ls",
			expected:    "",
			expectError: true,
			errorMsg:    "URL contains invalid characters",
		},
		{
			name:        "semicolon in fragment is rejected",
			raw:         "https://instagram.com/p/abc#;ls",
			expected:    "",
			expectError: true,
			errorMsg:    "URL contains invalid characters",
		},
		{
			name:        "fragment is stripped",
			raw:         "https://instagram.com/p/abc#fragment",
			expected:    "https://instagram.com/p/abc",
			expectError: false,
		},
		{
			name:        "multiple fragments are stripped",
			raw:         "https://instagram.com/p/abc#f1#f2",
			expected:    "https://instagram.com/p/abc",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validateURL(tt.raw)

			if tt.expectError {
				if err == nil {
					t.Fatalf("expected an error but got nil")
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("did not expect an error but got: %v", err)
				}
				if result != tt.expected {
					t.Errorf("expected %q, got %q", tt.expected, result)
				}
			}
		})
	}
}
