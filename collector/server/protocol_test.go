package server

import (
	"encoding/json"
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
		{&Request{Token: ``, Payload: json.RawMessage(``), CatcherType: ``}, args{false, "", "Token is empty"}},
		{&Request{Token: `token`, Payload: json.RawMessage(``), CatcherType: ``}, args{false, "", "CatcherType is empty"}},
		{&Request{Token: `token`, Payload: json.RawMessage(``), CatcherType: `qwe`}, args{false, "", "Invalid JWT signature"}},
		{&Request{Token: `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJwcm9qZWN0SWQiOiJ0ZXN0aWQiLCJpYXQiOjE1NjcwMzgxMDJ9.5FUrS-GY6jIToX_j6Y8gGoA-uVWiRS3o9w4AWyQPiqc`, Payload: json.RawMessage(``), CatcherType: `qwe`}, args{true, "testid", ""}},
	}

	for _, tt := range requestTests {
		flag, projectId, cause := tt.request.DecodeJWT()
		assert.Equal(t, flag, tt.expected.flag)
		assert.Equal(t, projectId, tt.expected.projectId)
		assert.Equal(t, cause, tt.expected.cause)
	}
}
