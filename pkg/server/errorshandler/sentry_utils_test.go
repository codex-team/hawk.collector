package errorshandler

import (
	"errors"
	"testing"
)

func TestGetSentryKeyFromAuth(t *testing.T) {
	tests := []struct {
		auth     string
		expected string
		err      error
	}{
		{"Sentry sentry_key=abc123, sentry_version=7", "abc123", nil},
		{"Sentry sentry_version=7, sentry_key=xyz789", "xyz789", nil},
		{"Sentry sentry_version=7", "", errors.New("sentry_key not found")},
		{"Sentry sentry_key=", "", nil},
		{"Sentry something_else=123", "", errors.New("sentry_key not found")},
		{"Sentry sentry_version=7,sentry_client=sentry.java.android/8.6.0,sentry_key=77e8ca0d39e3495fa7e360d960b76e5f789377f1fa2b4fe2bffb68649593a123", "77e8ca0d39e3495fa7e360d960b76e5f789377f1fa2b4fe2bffb68649593a123", nil},
	}

	for _, tt := range tests {
		result, err := getSentryKeyFromAuth(tt.auth)
		if result != tt.expected || (err != nil && err.Error() != tt.err.Error()) {
			t.Errorf("getSentryKeyFromAuth(%q) = (%q, %v), want (%q, %v)", tt.auth, result, err, tt.expected, tt.err)
		}
	}
}
