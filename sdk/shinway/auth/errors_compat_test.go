package auth_test

import (
	"net/http"
	"testing"

	shinwayauth "github.com/shinmentakezo07/shinway/v7/sdk/shinway/auth"
)

func TestErrorLegacyUnkeyedLiteralCompatibility(t *testing.T) {
	err := shinwayauth.Error{"code", "message", false, http.StatusRequestTimeout}

	if err.Code != "code" || err.Message != "message" || err.Retryable || err.HTTPStatus != http.StatusRequestTimeout {
		t.Fatalf("unexpected error fields: %#v", err)
	}
}
