# frozen_string_literal: true
#
# Error handling from Ruby.
#
# Any non-2xx response raises Dimail::APIError (a subclass of Dimail::Error <
# StandardError), carrying the status, the parsed `detail`, and predicates for
# the common cases.

require "dimail"

client = Dimail::Client.new(basic_auth: ["apiuser", "apipass"])
client.login

begin
  client.get_domain("absent.example")
rescue Dimail::APIError => e
  warn "API error #{e.status}: #{e.detail}"
  warn "no such domain" if e.not_found?
  raise unless e.not_found?
end

# Create a domain, tolerating a conflict if it already exists.
begin
  client.post_domain({ "name" => "new.gouv.fr", "features" => ["mailbox"] })
rescue Dimail::APIError => e
  raise unless e.conflict?
  warn "domain already exists"
end
