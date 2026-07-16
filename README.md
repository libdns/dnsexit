DNSExit for [`libdns`](https://github.com/libdns/libdns)
=======================

[![Go Reference](https://pkg.go.dev/badge/test.svg)](https://pkg.go.dev/github.com/libdns/dnsexit)

This package implements the [libdns interfaces](https://github.com/libdns/libdns) for DNSExit, allowing you to manage DNS records.

Configuration
=============

[DNSExit API documentation](https://dnsexit.com/dns/dns-api/) details the process of getting an API key.

To run clone the `.env_template` to a file named `.env` and populate with the API key and zone. Note that setting the environment variable 'LIBDNS_DNSEXIT_DEBUG=TRUE' will output the request body for debugging requests, and this will expose the API key.

Example
=======

```go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/libdns/dnsexit"
	"github.com/libdns/libdns"
)

func main() {
	key := os.Getenv("LIBDNS_DNSEXIT_API_KEY")
	if key == "" {
		fmt.Println("LIBDNS_DNSEXIT_API_KEY not set")
		return
	}

	zone := os.Getenv("LIBDNS_DNSEXIT_ZONE")
	if zone == "" {
		fmt.Println("LIBDNS_DNSEXIT_ZONE not set")
		return
	}

	p := &dnsexit.Provider{
		APIKey: key,
		// Optional explicit zone override. When set, this takes precedence
		// over the zone argument passed into libdns methods.
		Zone: zone,
	}

	records := []libdns.Record{
		{
			Type:  "A",
			Name:  "test",
			Value: "198.51.100.1",
		},
		{
			Type:  "AAAA",
			Name:  "test",
			Value: "2001:0db8::1",
		},
		{
			Type:  "TXT",
			Name:  "test",
			Value: "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
		},
	}

	ctx := context.Background()
	_, err := p.SetRecords(ctx, zone, records)
	if err != nil {
		fmt.Printf("Error: %v", err)
		return
	}
}
```

Caveats
=======

Live tests and rate limits
==========================

The `FOR_LIVE_TESTING` package performs real DNSExit API writes and can hit daily API limits.

- Default behavior: live tests are skipped unless explicitly enabled.
- Enable live tests only when needed:

```bash
LIBDNS_DNSEXIT_RUN_LIVE_TESTS=1 go test ./FOR_LIVE_TESTING -count=1
```

- Routine test runs can safely use:

```bash
go test ./...
```

with no live API calls unless `LIBDNS_DNSEXIT_RUN_LIVE_TESTS=1` is set.

Optional live-test naming overrides are available through `.env`:

- `LIBDNS_DNSEXIT_TEST_RECORD_PREFIX`: prefix for created record labels (default `libdns-live`).
- `LIBDNS_DNSEXIT_TEST_SUBDOMAIN`: subdomain under `LIBDNS_DNSEXIT_ZONE` used for live test names/targets (default `test_subdomain`).

This lets you keep live tests scoped under a dedicated subdomain while avoiding hardcoded domain-like values.

The API does not include a GET method, so fetching records is done via Google DNS. There will be some latency.

If an 'A' and 'AAAA' record have the same name, deleting either of them will remove both records. Note that deleting a record which does not exist returns an error from DNSExit, so we treat that as a fail also.

If multiple record updates are sent in one request, the API may return a code other than 0, to indicate partial success. This is currently judged as a fail and API error message is returned instead of the successfully amended records. 

When working with MX records, the mail server should be specified in the Target attribute. Also, any "Name" attribute will be ignored to avoid inconsistencies in the DNSExit API, (which ignores "name" when adding/updating, but needs it to contain the server name when deleting - this is handled by the library as long as the mail server is correctly specified in the Target). There is currently no documented way to use the API to specify a mail-subzone. See https://dnsexit.com/dns/dns-api/#example-update-mx

If your DNSExit account can update a delegated child zone but callers pass a different parent zone,
set `Provider.Zone` explicitly to your writable zone. An example of this is the free subdomains e,g, xxx.run.place. DNSExit won't allow record changes against run.place, but will against xxx.run.place.

If you are using this library through Caddy ACME DNS-01, configure recursive resolvers explicitly in Caddy
(for example `1.1.1.1 8.8.8.8`) rather than relying on local stub or split-DNS resolver paths.

For [Dynamic DNS](https://dnsexit.com/dns/dns-api/#dynamic-ip-update) DNSExit recommend their dedicated GET endpoint, which can set the domain's IP to the one making the request. That is not implemented in this library.
