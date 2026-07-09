<p align="center"><img src="https://raw.githubusercontent.com/go-ruby-dimail/brand/main/social.png" alt="go-ruby-dimail" width="720"></p>

# go-ruby-dimail

[![ci](https://github.com/go-ruby-dimail/dimail/actions/workflows/ci.yml/badge.svg)](https://github.com/go-ruby-dimail/dimail/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/go-ruby-dimail/dimail.svg)](https://pkg.go.dev/github.com/go-ruby-dimail/dimail)
[![Go Report Card](https://goreportcard.com/badge/github.com/go-ruby-dimail/dimail)](https://goreportcard.com/report/github.com/go-ruby-dimail/dimail)

The pure-Go, Ruby-runtime-independent core of the Ruby **`dimail`** gem — a
client for the **Dimail API** of the French government's *La Suite numérique*
platform — shaped so that [go-embedded-ruby](https://github.com/go-embedded-ruby/ruby)
(`rbgo`) can bind it as `require "dimail"`.

It is a thin, reflective adapter over the typed client in
[go-dimail/dimail](https://github.com/go-dimail/dimail). A `Session` exposes
**every one of the underlying client's 91 operations** through a single dynamic
entry point, `Call`, which:

1. maps a Ruby-style snake_case operation name (the API's `operationId`, e.g.
   `get_domain`, `post_mailbox_v2`) to the corresponding Go method;
2. coerces the arguments — Ruby Hashes become request structs, plain values
   become path and query parameters, omitted trailing arguments default to nil;
3. normalises the result into Ruby-shaped data: a **Hash** (`map[string]any`),
   an **Array** (`[]any`), or a scalar.

That uniform surface is exactly what an rbgo binding drives from
`method_missing`. Nothing here depends on the Ruby runtime, so it is equally
usable as a standalone Go library — a sibling of `go-ruby-regexp/regexp` and
`go-ruby-erb/erb`.

- **CGO-free**, builds and tests identically on `amd64`, `arm64`, `riscv64`,
  `loong64`, `ppc64le` and `s390x`.
- **100 % test coverage**, race-clean, enforced in CI.

## Usage from Ruby

This is the Ruby gem's core: under rbgo, `require "dimail"` gives you a
`Dimail::Client` whose snake_case methods are the API operations, returning Ruby
Hashes and Arrays. See [`examples/`](examples) for runnable scripts and the full
surface contract.

```ruby
require "dimail"

client = Dimail::Client.new(basic_auth: ["apiuser", "apipass"])
client.login

domain = client.get_domain("example.gouv.fr")   # => Hash
puts domain["state"]

client.get_domains.each { |d| puts d["name"] }   # => Array<Hash>

client.post_mailbox_v2("example.gouv.fr", "jean.dupont",
                       { "features" => ["ox"] })

begin
  client.get_domain("absent.example")
rescue Dimail::APIError => e
  warn "not found" if e.not_found?
end
```

The `require "dimail"` binding lives in rbgo (a thin `method_missing` shim over
the `Session` below); it is pending in that repo. The operation names in the
examples are checked against the real generated API by a Go test in this package,
so they cannot drift.

## Install (Go)

```sh
go get github.com/go-ruby-dimail/dimail
```

## Usage from Go

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/go-ruby-dimail/dimail"
)

func main() {
	ctx := context.Background()

	s := dimail.NewSession(dimail.WithBasicAuth("apiuser", "apipass"))
	if _, err := s.Login(ctx); err != nil { // stores a bearer token
		log.Fatal(err)
	}

	// A single object comes back as a Hash.
	dom, err := s.Call(ctx, "get_domain", "example.gouv.fr")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(dom.(map[string]any)["state"])

	// A collection comes back as an Array of Hashes.
	all, err := s.Call(ctx, "get_domains")
	if err != nil {
		log.Fatal(err)
	}
	for _, d := range all.([]any) {
		fmt.Println(d.(map[string]any)["name"])
	}

	// A request body is passed as a Hash; query parameters as plain values.
	_, err = s.Call(ctx, "post_mailbox_v2", "example.gouv.fr", "jean.dupont",
		map[string]any{"features": []string{"ox"}})
	if err != nil {
		log.Fatal(err)
	}
}
```

`Session.Methods()` lists every snake_case name `Call` accepts.

## Relationship to go-dimail

| Repo | Role |
| --- | --- |
| [`go-dimail/dimail`](https://github.com/go-dimail/dimail) | The typed, OpenAPI-generated Go client (transport, models, errors). |
| `go-ruby-dimail/dimail` | This repo — the Ruby-idiomatic, Hash-returning adapter that rbgo binds. |

`Session.Client()` returns the underlying `*godimail.Client` for callers who
want the fully typed API.

## License

BSD-3-Clause. See [LICENSE](LICENSE).

This is an independent client library; it is not affiliated with or endorsed by
DINUM or the *La Suite numérique* team.
