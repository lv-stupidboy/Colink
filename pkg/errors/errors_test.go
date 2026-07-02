package errors

import (
	stderrors "errors"
	"net/http"
	"strings"
	"testing"
)

func TestAppErrorHelpers(t *testing.T) {
	err := WithDetail(ErrInvalidParam, "bad field")
	if err.Code != CodeInvalidParam || err.Message != ErrInvalidParam.Message || err.Error() != "bad field" {
		t.Fatalf("unexpected detail error: %#v", err)
	}
	if WrapError(nil) != nil {
		t.Fatal("expected nil error to wrap as nil")
	}
	if WrapError(err) != err {
		t.Fatal("expected AppError to be returned as-is")
	}
	wrapped := WrapError(stderrors.New("boom"))
	if wrapped.Code != CodeInternal || wrapped.Detail != "boom" {
		t.Fatalf("unexpected wrapped error: %#v", wrapped)
	}
}

func TestWrapGitErrorClassifiesKnownFailures(t *testing.T) {
	base := stderrors.New("git failed")
	tests := []struct {
		name string
		out  string
		code ErrorCode
	}{
		{name: "timeout", out: "context deadline exceeded", code: CodeNetworkTimeout},
		{name: "auth", out: "Permission denied (publickey)", code: CodeAuthFailed},
		{name: "repo", out: "remote: Repository not found", code: CodeRepoNotFound},
		{name: "branch", out: "Remote branch feature not found", code: CodeBranchNotFound},
		{name: "unknown", out: "fatal: something else", code: CodeInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WrapGitError(tt.out, base)
			if got.Code != tt.code {
				t.Fatalf("code=%s, want %s", got.Code, tt.code)
			}
			if !strings.Contains(got.Detail, base.Error()) || !strings.Contains(got.Detail, tt.out) {
				t.Fatalf("detail should include command error and output, got %q", got.Detail)
			}
		})
	}
	if WrapGitError("ignored", nil) != nil {
		t.Fatal("expected nil git error to wrap as nil")
	}
}

func TestHTTPStatusAndConstructors(t *testing.T) {
	statuses := map[ErrorCode]int{
		CodeNetworkTimeout:  http.StatusServiceUnavailable,
		CodeAuthFailed:      http.StatusUnauthorized,
		CodeRepoNotFound:    http.StatusNotFound,
		CodeBranchNotFound:  http.StatusNotFound,
		CodePackageNotFound: http.StatusNotFound,
		CodeInvalidParam:    http.StatusBadRequest,
		CodeInternal:        http.StatusInternalServerError,
	}
	for code, want := range statuses {
		if got := ToHTTPStatus(code); got != want {
			t.Fatalf("status for %s=%d, want %d", code, got, want)
		}
	}
	if got := ToHTTPStatus("OTHER"); got != http.StatusInternalServerError {
		t.Fatalf("unknown status=%d", got)
	}
	if got := NewInvalidParam("missing name"); got.Code != CodeInvalidParam || got.Detail != "missing name" {
		t.Fatalf("unexpected invalid param: %#v", got)
	}
	if got := NewPackageNotFound("team"); got.Code != CodePackageNotFound || !strings.Contains(got.Detail, "team") {
		t.Fatalf("unexpected package not found: %#v", got)
	}
	if got := NewParseFailed("manifest.json", stderrors.New("bad json")); got.Code != CodeParseFailed || !strings.Contains(got.Detail, "manifest.json") {
		t.Fatalf("unexpected parse failed: %#v", got)
	}
}
