package dimail

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// mockServer replies with a body that tests set per call, recording the path.
type mockServer struct {
	status  int
	body    string
	gotPath string
	gotBody string
}

func (m *mockServer) start(t *testing.T) *Session {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.gotPath = r.URL.RequestURI()
		b, _ := io.ReadAll(r.Body)
		m.gotBody = string(b)
		if m.status != 0 {
			w.WriteHeader(m.status)
		}
		_, _ = io.WriteString(w, m.body)
	}))
	t.Cleanup(ts.Close)
	return NewSession(WithBaseURL(ts.URL), WithToken("t"))
}

func TestNewSessionAndClient(t *testing.T) {
	s := NewSession(WithBasicAuth("u", "p"), WithUserAgent("ua"))
	if s.Client() == nil {
		t.Fatal("Client() nil")
	}
}

func TestLogin(t *testing.T) {
	m := &mockServer{body: `{"access_token":"TK","token_type":"bearer"}`}
	s := m.start(t)
	h, err := s.Login(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if h["access_token"] != "TK" {
		t.Fatalf("login hash = %v", h)
	}
	if s.Client().CurrentToken() != "TK" {
		t.Fatal("token not stored")
	}
}

func TestLoginError(t *testing.T) {
	m := &mockServer{status: http.StatusUnauthorized, body: `{"detail":"nope"}`}
	s := m.start(t)
	if _, err := s.Login(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestCallHashReturn(t *testing.T) {
	m := &mockServer{body: `{"name":"example.gouv.fr","state":"ok","delivery":"virtual","features":["mailbox"],"mx_name":"internal"}`}
	s := m.start(t)
	got, err := s.Call(context.Background(), "get_domain", "example.gouv.fr")
	if err != nil {
		t.Fatal(err)
	}
	h, ok := got.(map[string]any)
	if !ok || h["name"] != "example.gouv.fr" {
		t.Fatalf("hash = %#v", got)
	}
	if m.gotPath != "/domains/example.gouv.fr" {
		t.Fatalf("path = %q", m.gotPath)
	}
}

func TestCallArrayReturn(t *testing.T) {
	m := &mockServer{body: `[{"name":"a.fr","state":"ok","delivery":"virtual","features":[],"mx_name":"internal"}]`}
	s := m.start(t)
	got, err := s.Call(context.Background(), "get_domains")
	if err != nil {
		t.Fatal(err)
	}
	arr, ok := got.([]any)
	if !ok || len(arr) != 1 {
		t.Fatalf("array = %#v", got)
	}
}

func TestCallVoidReturn(t *testing.T) {
	m := &mockServer{body: ``}
	s := m.start(t)
	got, err := s.Call(context.Background(), "delete_user", "bob")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("void call returned %#v", got)
	}
}

func TestCallBodyFromHash(t *testing.T) {
	m := &mockServer{body: `{"name":"bob","is_admin":false,"uuid":"u","perms":[],"acls":[],"identities":[]}`}
	s := m.start(t)
	_, err := s.Call(context.Background(), "post_user", map[string]any{
		"name": "bob", "password": "pw", "is_admin": false, "perms": []string{"x"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m.gotBody, `"name":"bob"`) {
		t.Fatalf("body = %q", m.gotBody)
	}
}

func TestCallQueryParamValueAndNil(t *testing.T) {
	m := &mockServer{body: `{"access_token":"x","token_type":"bearer"}`}
	s := m.start(t)
	// Value coerced string -> *string.
	if _, err := s.Call(context.Background(), "get_token", "alice"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m.gotPath, "username=alice") {
		t.Fatalf("path = %q", m.gotPath)
	}
	// Omitted argument -> nil pointer -> parameter absent.
	if _, err := s.Call(context.Background(), "get_token"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(m.gotPath, "username=") {
		t.Fatalf("path should omit username: %q", m.gotPath)
	}
}

func TestCallAssignableArg(t *testing.T) {
	m := &mockServer{body: `{"access_token":"x","token_type":"bearer"}`}
	s := m.start(t)
	user := "carol"
	// A *string is directly assignable to the *string parameter.
	if _, err := s.Call(context.Background(), "get_token", &user); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m.gotPath, "username=carol") {
		t.Fatalf("path = %q", m.gotPath)
	}
}

func TestCallStringCoercionFromNonString(t *testing.T) {
	m := &mockServer{body: `{"name":"123","state":"ok","delivery":"virtual","features":[],"mx_name":"internal"}`}
	s := m.start(t)
	// A non-string path argument is stringified.
	if _, err := s.Call(context.Background(), "get_domain", 123); err != nil {
		t.Fatal(err)
	}
	if m.gotPath != "/domains/123" {
		t.Fatalf("path = %q", m.gotPath)
	}
}

func TestCallErrors(t *testing.T) {
	m := &mockServer{status: http.StatusInternalServerError, body: `{"detail":"x"}`}
	s := m.start(t)
	ctx := context.Background()

	if _, err := s.Call(ctx, "no_such_method"); err == nil ||
		!strings.Contains(err.Error(), "unknown method") {
		t.Fatalf("want unknown method, got %v", err)
	}
	if _, err := s.Call(ctx, "current_token"); err == nil ||
		!strings.Contains(err.Error(), "not a request method") {
		t.Fatalf("want not-a-request-method, got %v", err)
	}
	if _, err := s.Call(ctx, "get_version", "extra"); err == nil ||
		!strings.Contains(err.Error(), "at most") {
		t.Fatalf("want too-many-args, got %v", err)
	}
	// Body coercion failure: a string cannot decode into a struct pointer.
	if _, err := s.Call(ctx, "post_user", "not-a-hash"); err == nil ||
		!strings.Contains(err.Error(), "argument") {
		t.Fatalf("want coercion error, got %v", err)
	}
	// Underlying call returns an error (500).
	if _, err := s.Call(ctx, "get_version"); err == nil {
		t.Fatal("want transport error")
	}
}

func TestCoerceDirect(t *testing.T) {
	strPtr := reflect.TypeOf((*string)(nil))
	strT := reflect.TypeOf("")
	intT := reflect.TypeOf(0)

	// A nil value for a required string parameter becomes the empty string.
	if v, err := coerce(nil, strT); err != nil || v.Interface().(string) != "" {
		t.Fatalf("coerce(nil, string) = %v, %v", v, err)
	}
	// A plain string coerced against a *named* string target (not directly
	// assignable) takes the string-assertion path.
	type namedStr string
	if v, err := coerce("hi", reflect.TypeOf(namedStr(""))); err != nil ||
		v.String() != "hi" {
		t.Fatalf("coerce(string, namedStr) = %v, %v", v, err)
	}

	// *string from a non-string value uses fmt.Sprint.
	v, err := coerce(7, strPtr)
	if err != nil {
		t.Fatal(err)
	}
	if got := v.Interface().(*string); *got != "7" {
		t.Fatalf("*string = %q", *got)
	}
	// default branch: JSON into a non-pointer, non-string target.
	v, err = coerce(float64(5), intT)
	if err != nil {
		t.Fatal(err)
	}
	if v.Interface().(int) != 5 {
		t.Fatalf("int = %v", v.Interface())
	}
	// default branch error: an unmarshalable value.
	if _, err := coerce(make(chan int), intT); err == nil {
		t.Fatal("want marshal error in default branch")
	}
	// pointer-struct branch marshal error.
	if _, err := coerce(make(chan int), reflect.TypeOf((*struct{ X int })(nil))); err == nil {
		t.Fatal("want marshal error in pointer branch")
	}
}

func TestToRubyDirect(t *testing.T) {
	if v, err := toRuby(nil); v != nil || err != nil {
		t.Fatalf("toRuby(nil) = %v, %v", v, err)
	}
	var np *struct{ X int }
	if v, err := toRuby(np); v != nil || err != nil {
		t.Fatalf("toRuby(nil ptr) = %v, %v", v, err)
	}
	if _, err := toRuby(make(chan int)); err == nil {
		t.Fatal("want marshal error")
	}
	v, err := toRuby(struct {
		X int `json:"x"`
	}{X: 3})
	if err != nil {
		t.Fatal(err)
	}
	if v.(map[string]any)["x"].(float64) != 3 {
		t.Fatalf("toRuby struct = %#v", v)
	}
}

func TestMethods(t *testing.T) {
	s := NewSession()
	ms := s.Methods()
	var hasGetDomain, hasPostMailboxV2 bool
	for _, m := range ms {
		switch m {
		case "get_domain":
			hasGetDomain = true
		case "post_mailbox_v2":
			hasPostMailboxV2 = true
		}
	}
	if !hasGetDomain || !hasPostMailboxV2 {
		t.Fatalf("Methods() missing expected names: %v", ms)
	}
	// Non-request methods (no context first arg) must be excluded.
	for _, m := range ms {
		if m == "current_token" || m == "set_token" {
			t.Fatalf("Methods() should exclude %q", m)
		}
	}
}

func TestNameHelpers(t *testing.T) {
	// initialism handling in camelize.
	if got := camelize("get_ox_id"); got != "GetOXID" {
		t.Fatalf("camelize = %q", got)
	}
	if got := camelize("post_mailbox_v2"); got != "PostMailboxV2" {
		t.Fatalf("camelize = %q", got)
	}
	// snakeize acronym boundary (uppercase run followed by a lowercase).
	if got := snakeize("HTTPServer"); got != "http_server" {
		t.Fatalf("snakeize = %q", got)
	}
	if got := snakeize("GetMailboxesV2"); got != "get_mailboxes_v2" {
		t.Fatalf("snakeize = %q", got)
	}
}
