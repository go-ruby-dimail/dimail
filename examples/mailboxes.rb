# frozen_string_literal: true
#
# Managing mailboxes and forwards from Ruby.
#
# Path parameters are passed as plain values, request bodies as Hashes, and
# optional query parameters may simply be omitted.

require "dimail"

client = Dimail::Client.new(token: ENV["DIMAIL_TOKEN"])

# Create a mailbox (v2 API): two path parameters, then the body Hash.
created = client.post_mailbox_v2("example.gouv.fr", "jean.dupont",
                                 { "features" => ["ox"] })
puts "created #{created["email"]} (password: #{created["password"]})"

# List every mailbox on a domain, then fetch one.
client.get_mailboxes_v2("example.gouv.fr").each do |mb|
  puts mb["email"]
end
one = client.get_mailbox_v2("example.gouv.fr", "jean.dupont")
puts one["status"]

# Update a mailbox.
client.patch_mailbox_v2("example.gouv.fr", "jean.dupont",
                        { "additionalSenders" => ["team@example.gouv.fr"] })

# Forwards: create one, then list them.
client.post_forward("example.gouv.fr", "contact",
                    { "nexthop" => "team@other.example" })
client.get_forwards("example.gouv.fr").each { |f| puts f["nexthop"] }
