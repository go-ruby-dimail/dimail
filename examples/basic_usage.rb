# frozen_string_literal: true
#
# Basic usage of the Dimail client from Ruby.
#
# Runs under go-embedded-ruby (rbgo) once the `require "dimail"` binding ships;
# see examples/README.md for the surface contract and status.

require "dimail"

# Authenticate with HTTP Basic credentials and exchange them for a bearer token
# (stored on the client for subsequent calls).
client = Dimail::Client.new(basic_auth: ["apiuser", "apipass"])
client.login

# A single resource comes back as a Hash keyed by the API's JSON field names.
domain = client.get_domain("example.gouv.fr")
puts "#{domain["name"]}: #{domain["state"]}"

# A collection comes back as an Array of Hashes.
client.get_domains.each do |d|
  puts d["name"]
end

# Self-service views for the authenticated user.
client.get_my_domains.each { |d| puts d["name"] }

# System endpoints work the same way.
puts client.get_version["version"]
