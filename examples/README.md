# Ruby examples

These are pure-Ruby examples of the `dimail` gem — the Ruby face of this
library. They run under [go-embedded-ruby](https://github.com/go-embedded-ruby/ruby)
(rbgo) once its `require "dimail"` binding is installed; that binding is a thin
`method_missing` shim over the `Session` in this repo (see the top-level README).

| File | Shows |
| --- | --- |
| [`basic_usage.rb`](basic_usage.rb) | Login, fetch a domain (Hash), list domains (Array), self-service views. |
| [`mailboxes.rb`](mailboxes.rb) | Create/list/fetch/update mailboxes (v2) and forwards. |
| [`error_handling.rb`](error_handling.rb) | Rescuing `Dimail::APIError` and its predicates. |

## Ruby surface contract

The rbgo binding exposes exactly this surface, backed by the Go `Session`:

```ruby
require "dimail"

# Construction. Keyword options map to the Go client options:
#   base_url:   -> WithBaseURL      token:      -> WithToken
#   basic_auth: -> WithBasicAuth    user_agent: -> WithUserAgent  (an [user, pass] pair)
client = Dimail::Client.new(basic_auth: ["user", "pass"])

client.login                              # -> Hash (the token); stores the bearer token

# Every API operation is a snake_case method named after its OpenAPI operationId.
# Dispatch is dynamic (method_missing -> Session#Call):
#   * path parameters  -> plain positional arguments
#   * query parameters -> trailing positional arguments (omit to leave unset)
#   * request body     -> a Hash
# Results are normalised to Ruby data: a Hash, an Array of Hashes, or a scalar.
client.get_domain("example.gouv.fr")                       # -> Hash
client.get_domains                                         # -> Array<Hash>
client.post_mailbox_v2("d.fr", "user", { "features" => ["ox"] })
```

Every operation name accepted here is the snake_case form of an `operationId`
in [`../openapi.json`](../openapi.json); `Session#Methods` (Ruby: the binding's
method list) enumerates them.

### Errors

```ruby
Dimail::Error      < StandardError   # base
Dimail::APIError   < Dimail::Error   # any non-2xx response
```

`Dimail::APIError` carries `#status` (Integer), `#detail` (the parsed FastAPI
`detail`, a String/Array/Hash or nil), `#body` (raw String), and the predicates
`#not_found?` (404), `#unauthorized?` (401), `#forbidden?` (403), `#conflict?`
(409) — mirroring `APIError` in the Go client.

## Verification

The operation names used in these files are checked against the real, generated
operation set by a Go test in this package
(`TestRubyExamplesReferenceRealOperations`), so the examples cannot drift from
the API. End-to-end execution is exercised in rbgo's own test suite once the
`require "dimail"` binding lands.
