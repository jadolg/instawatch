package main

import (
	"strings"
	"testing"
)

func TestValidateInstagramURL(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		expected    string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid https URL",
			raw:         "https://instagram.com/p/CLxyz",
			expected:    "https://instagram.com/p/CLxyz",
			expectError: false,
		},
		{
			name:        "valid https www URL",
			raw:         "https://www.instagram.com/reel/CLxyz",
			expected:    "https://www.instagram.com/reel/CLxyz",
			expectError: false,
		},
		{
			name:        "missing scheme prepends https",
			raw:         "instagram.com/p/CLxyz",
			expected:    "https://instagram.com/p/CLxyz",
			expectError: false,
		},
		{
			name:        "missing scheme with www prepends https",
			raw:         "www.instagram.com/reel/CLxyz",
			expected:    "https://www.instagram.com/reel/CLxyz",
			expectError: false,
		},
		{
			name:        "collapsed https single slash",
			raw:         "https:/instagram.com/p/CLxyz",
			expected:    "https://instagram.com/p/CLxyz",
			expectError: false,
		},
		{
			name:        "strips query parameters",
			raw:         "https://instagram.com/p/CLxyz?igshid=123&utm_source=wa",
			expected:    "https://instagram.com/p/CLxyz",
			expectError: false,
		},
		{
			name:        "strips query parameters when no scheme initially",
			raw:         "instagram.com/p/CLxyz?igshid=123",
			expected:    "https://instagram.com/p/CLxyz",
			expectError: false,
		},
		{
			name:        "strips fragment",
			raw:         "https://instagram.com/p/CLxyz#fragment",
			expected:    "https://instagram.com/p/CLxyz",
			expectError: false,
		},
		{
			name:        "strips both query and fragment",
			raw:         "https://instagram.com/p/CLxyz?igshid=123#fragment",
			expected:    "https://instagram.com/p/CLxyz",
			expectError: false,
		},
		{
			name:        "http scheme is rejected",
			raw:         "http://instagram.com/p/CLxyz",
			expected:    "",
			expectError: true,
			errorMsg:    "only HTTPS Instagram URLs are accepted",
		},
		{
			name:        "collapsed http scheme is rejected",
			raw:         "http:/instagram.com/p/CLxyz",
			expected:    "",
			expectError: true,
			errorMsg:    "only HTTPS Instagram URLs are accepted",
		},
		{
			name:        "unsupported scheme is rejected",
			raw:         "ftp://instagram.com/p/CLxyz",
			expected:    "",
			expectError: true,
			errorMsg:    "only HTTPS Instagram URLs are accepted",
		},
		{
			name:        "invalid host tiktok",
			raw:         "https://tiktok.com/v/123",
			expected:    "",
			expectError: true,
			errorMsg:    "not a valid Instagram URL",
		},
		{
			name:        "invalid host subdomain mismatch",
			raw:         "https://m.instagram.com/p/CLxyz",
			expected:    "",
			expectError: true,
			errorMsg:    "not a valid Instagram URL",
		},
		{
			name:        "invalid host malicious domain suffix",
			raw:         "https://instagram.com.evil.com/p/123",
			expected:    "",
			expectError: true,
			errorMsg:    "not a valid Instagram URL",
		},
		{
			name:        "invalid host malicious domain prefix",
			raw:         "https://fakeinstagram.com/p/123",
			expected:    "",
			expectError: true,
			errorMsg:    "not a valid Instagram URL",
		},
		{
			name:        "empty string",
			raw:         "",
			expected:    "",
			expectError: true,
			errorMsg:    "not a valid Instagram URL",
		},
		{
			name:        "invalid url structure that fails url.Parse",
			raw:         "https://\x7f\x81zzzzz",
			expected:    "",
			expectError: true,
			errorMsg:    "invalid URL",
		},
		{
			name:        "url with port",
			raw:         "https://instagram.com:443/p/CLxyz",
			expected:    "https://instagram.com:443/p/CLxyz",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validateInstagramURL(tt.raw)

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
