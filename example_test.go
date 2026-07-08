package dimail_test

import (
	"context"
	"fmt"
	"log"

	"github.com/go-ruby-dimail/dimail"
)

// This example mirrors the README. It is compiled (guaranteeing the documented
// API stays valid) but not run, since it would contact the live service.
func Example() {
	ctx := context.Background()

	s := dimail.NewSession(dimail.WithBasicAuth("apiuser", "apipass"))
	if _, err := s.Login(ctx); err != nil { // stores a bearer token
		log.Fatal(err)
	}

	dom, err := s.Call(ctx, "get_domain", "example.gouv.fr")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(dom.(map[string]any)["state"])

	all, err := s.Call(ctx, "get_domains")
	if err != nil {
		log.Fatal(err)
	}
	for _, d := range all.([]any) {
		fmt.Println(d.(map[string]any)["name"])
	}

	_, err = s.Call(ctx, "post_mailbox_v2", "example.gouv.fr", "jean.dupont",
		map[string]any{"features": []string{"ox"}})
	if err != nil {
		log.Fatal(err)
	}
}
