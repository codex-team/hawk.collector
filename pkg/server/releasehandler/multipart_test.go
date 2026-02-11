package releasehandler

import (
	"mime/multipart"
	"testing"
)

func TestGetSingleFormValue(t *testing.T) {
	form := &multipart.Form{Value: map[string][]string{}}

	if err, _ := getSingleFormValue(form, "release"); err == nil {
		t.Fatalf("expected error for missing value")
	}

	form.Value["release"] = []string{"a", "b"}
	if err, _ := getSingleFormValue(form, "release"); err == nil {
		t.Fatalf("expected error for multiple values")
	}

	form.Value["release"] = []string{"v1.2.3"}
	if err, value := getSingleFormValue(form, "release"); err != nil || value != "v1.2.3" {
		t.Fatalf("expected value v1.2.3, got %q err=%v", value, err)
	}
}
