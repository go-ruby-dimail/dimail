// Package dimail is the pure-Go, Ruby-runtime-independent core of the Ruby
// `dimail` gem: a client for the Dimail API of the French government's "La Suite
// numérique" platform, shaped so that github.com/go-embedded-ruby/ruby can bind
// it as `require "dimail"`.
//
// It is a thin, reflective adapter over the typed client in
// github.com/go-dimail/dimail. A Session exposes every one of the underlying
// client's operations through a single dynamic entry point, Call, which maps a
// Ruby-style snake_case method name to the corresponding Go method, coerces the
// arguments (Ruby Hashes become request structs, plain values become path and
// query parameters), and normalises the result into Ruby-shaped values — a Hash
// (map[string]any), an Array ([]any) or a scalar. That uniform surface is what
// an rbgo binding drives from `method_missing`; nothing here depends on the Ruby
// runtime, so it is equally usable as a standalone Go library.
//
//	s := dimail.NewSession(dimail.WithBasicAuth("user", "pass"))
//	if _, err := s.Login(ctx); err != nil {
//		return err
//	}
//	dom, err := s.Call(ctx, "get_domain", "example.gouv.fr") // dom is a Hash
//	all, err := s.Call(ctx, "get_domains")                    // all is an Array
package dimail

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"unicode"

	godimail "github.com/go-dimail/dimail"
)

// Option configures the underlying client. The options mirror go-dimail so that
// callers (and the rbgo binding) need not import both packages.
type Option = godimail.Option

// Re-exported client options.
var (
	WithBaseURL    = godimail.WithBaseURL
	WithHTTPClient = godimail.WithHTTPClient
	WithBasicAuth  = godimail.WithBasicAuth
	WithToken      = godimail.WithToken
	WithUserAgent  = godimail.WithUserAgent
)

// Session is a Ruby-facing handle over a go-dimail client.
type Session struct {
	c *godimail.Client
}

// NewSession builds a Session pointed at the production API by default.
func NewSession(opts ...Option) *Session {
	return &Session{c: godimail.NewClient(opts...)}
}

// Client exposes the underlying typed client for callers that want it.
func (s *Session) Client() *godimail.Client { return s.c }

// Login obtains and stores a bearer token from the configured Basic
// credentials, returning the token as a Ruby Hash.
func (s *Session) Login(ctx context.Context) (map[string]any, error) {
	tok, err := s.c.Login(ctx)
	if err != nil {
		return nil, err
	}
	// tok is a *Token, which always marshals; the error is structurally
	// impossible here.
	ruby, _ := toRuby(tok)
	h, _ := ruby.(map[string]any)
	return h, nil
}

var (
	contextType = reflect.TypeOf((*context.Context)(nil)).Elem()
	errorType   = reflect.TypeOf((*error)(nil)).Elem()
)

// Call dispatches a Ruby-style snake_case operation name (matching the API's
// operationId, e.g. "get_domain", "post_mailbox_v2") to the underlying client
// method. Path parameters are passed as plain values, query parameters as
// values or nil to omit them, and a request body as a Ruby Hash. Trailing
// arguments may be omitted; they default to nil. The result is normalised to a
// Ruby-shaped value (Hash, Array or scalar), or nil for operations with no body.
func (s *Session) Call(ctx context.Context, method string, args ...any) (any, error) {
	m := reflect.ValueOf(s.c).MethodByName(camelize(method))
	if !m.IsValid() {
		return nil, fmt.Errorf("dimail: unknown method %q", method)
	}
	mt := m.Type()
	if mt.NumIn() == 0 || mt.In(0) != contextType {
		return nil, fmt.Errorf("dimail: %q is not a request method", method)
	}

	// In(0) is the context; the remaining inputs are the caller's arguments.
	want := mt.NumIn() - 1
	if len(args) > want {
		return nil, fmt.Errorf("dimail: %s takes at most %d argument(s), got %d",
			method, want, len(args))
	}

	in := make([]reflect.Value, mt.NumIn())
	in[0] = reflect.ValueOf(ctx)
	for i := 1; i < mt.NumIn(); i++ {
		var arg any
		if i-1 < len(args) {
			arg = args[i-1]
		}
		v, err := coerce(arg, mt.In(i))
		if err != nil {
			return nil, fmt.Errorf("dimail: %s argument %d: %w", method, i, err)
		}
		in[i] = v
	}

	outs := m.Call(in)

	// A trailing error output is unwrapped; everything else is a result.
	results := outs
	var callErr error
	if n := len(outs); n > 0 && mt.Out(n-1) == errorType {
		if e := outs[n-1]; !e.IsNil() {
			callErr = e.Interface().(error)
		}
		results = outs[:n-1]
	}
	if callErr != nil {
		return nil, callErr
	}
	if len(results) == 0 {
		return nil, nil
	}
	return toRuby(results[0].Interface())
}

// Methods lists the Ruby-style names Call accepts, sorted.
func (s *Session) Methods() []string {
	t := reflect.TypeOf(s.c)
	names := make([]string, 0, t.NumMethod())
	for i := 0; i < t.NumMethod(); i++ {
		mt := t.Method(i).Func.Type()
		// Only expose request-shaped methods: (receiver, context.Context, ...).
		if mt.NumIn() >= 2 && mt.In(1) == contextType {
			names = append(names, snakeize(t.Method(i).Name))
		}
	}
	sort.Strings(names)
	return names
}

// coerce converts a Ruby-supplied argument into the Go type target expects.
func coerce(val any, target reflect.Type) (reflect.Value, error) {
	if val != nil {
		if vv := reflect.ValueOf(val); vv.Type().AssignableTo(target) {
			return vv, nil
		}
	}
	switch target.Kind() {
	case reflect.String:
		if val == nil {
			return reflect.ValueOf(""), nil
		}
		if s, ok := val.(string); ok {
			return reflect.ValueOf(s), nil
		}
		return reflect.ValueOf(fmt.Sprint(val)), nil
	case reflect.Pointer:
		if val == nil {
			return reflect.Zero(target), nil
		}
		p := reflect.New(target.Elem())
		if target.Elem().Kind() == reflect.String {
			if s, ok := val.(string); ok {
				p.Elem().SetString(s)
			} else {
				p.Elem().SetString(fmt.Sprint(val))
			}
			return p, nil
		}
		if err := jsonInto(val, p.Interface()); err != nil {
			return reflect.Value{}, err
		}
		return p, nil
	default:
		p := reflect.New(target)
		if err := jsonInto(val, p.Interface()); err != nil {
			return reflect.Value{}, err
		}
		return p.Elem(), nil
	}
}

// jsonInto round-trips val through JSON into the pointer dst, giving a faithful
// Ruby-Hash-to-struct conversion.
func jsonInto(val any, dst any) error {
	b, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}

// toRuby normalises a typed Go value into Ruby-shaped data (Hash/Array/scalar).
func toRuby(v any) (any, error) {
	if v == nil {
		return nil, nil
	}
	if rv := reflect.ValueOf(v); rv.Kind() == reflect.Pointer && rv.IsNil() {
		return nil, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out any
	// b is JSON produced by json.Marshal above, so it always decodes.
	_ = json.Unmarshal(b, &out)
	return out, nil
}

// camelize turns a snake_case operationId into the exported Go method name,
// matching the naming used by the go-dimail generator.
func camelize(s string) string {
	var b strings.Builder
	for _, w := range splitWords(s) {
		if initialisms[strings.ToUpper(w)] {
			b.WriteString(strings.ToUpper(w))
			continue
		}
		r := []rune(w)
		r[0] = unicode.ToUpper(r[0])
		b.WriteString(string(r))
	}
	return b.String()
}

// snakeize is the inverse: an exported Go method name to snake_case.
func snakeize(s string) string {
	var b []rune
	rs := []rune(s)
	for i, r := range rs {
		if unicode.IsUpper(r) {
			prevLower := i > 0 && unicode.IsLower(rs[i-1])
			nextLower := i+1 < len(rs) && unicode.IsLower(rs[i+1])
			if i > 0 && (prevLower || nextLower) {
				b = append(b, '_')
			}
			r = unicode.ToLower(r)
		}
		b = append(b, r)
	}
	return string(b)
}

func splitWords(s string) []string {
	var words []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			words = append(words, cur.String())
			cur.Reset()
		}
	}
	for _, r := range s {
		if r == '_' || r == '-' || r == ' ' {
			flush()
			continue
		}
		cur.WriteRune(r)
	}
	flush()
	return words
}

var initialisms = map[string]bool{
	"ID": true, "UUID": true, "URL": true, "URI": true, "API": true, "HTTP": true,
	"MX": true, "IMAP": true, "SMTP": true, "ACL": true, "ACLS": true, "SPF": true,
	"DKIM": true, "OX": true, "DNS": true, "TTL": true, "IP": true, "JSON": true,
	"DB": true, "OK": true, "OIDC": true,
}
