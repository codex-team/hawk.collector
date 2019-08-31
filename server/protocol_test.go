package server

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProtocolVerification(t *testing.T) {

	type args struct {
		flag      bool
		projectId string
		cause     string
	}

	jwtSecret = "qwerty"

	requestTests := []struct {
		request  *Request
		expected args
	}{
		{&Request{Token: `token`, Payload: json.RawMessage(``), CatcherType: `qwe`}, args{false, "", "Invalid JWT signature"}},
		{&Request{Token: `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.350o18fZPeOi3tEGEac6U4UzuB_k-FuZeVQvzf369IQ`, Payload: json.RawMessage(``), CatcherType: `qwe`}, args{true, "", "Empty projectId"}},
		{&Request{Token: `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJwcm9qZWN0SWQiOiJ0ZXN0aWQiLCJpYXQiOjE1NjcwMzgxMDJ9.5FUrS-GY6jIToX_j6Y8gGoA-uVWiRS3o9w4AWyQPiqc`, Payload: json.RawMessage(``), CatcherType: `qwe`}, args{true, "testid", "<nil>"}},
	}

	for _, tt := range requestTests {
		projectId, err := DecodeJWT(tt.request.Token)
		assert.Equal(t, projectId, tt.expected.projectId)
		assert.Equal(t, fmt.Sprintf("%v", err), tt.expected.cause)
	}
}
