package server

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProtocolVerification(t *testing.T) {

	type args struct {
		flag  bool
		cause string
	}

	requestTests := []struct {
		request  *Request
		expected args
	}{
		{&Request{Token: ``, Payload: json.RawMessage(``), CatcherType: ``}, args{false, "Token is empty"}},
		{&Request{Token: `token`, Payload: json.RawMessage(``), CatcherType: ``}, args{false, "CatcherType is empty"}},
		{&Request{Token: `token`, Payload: json.RawMessage(``), CatcherType: `qwe`}, args{true, ""}},
	}

	for _, tt := range requestTests {
		flag, cause := tt.request.Validate()
		assert.Equal(t, flag, tt.expected.flag)
		assert.Equal(t, cause, tt.expected.cause)
	}
}