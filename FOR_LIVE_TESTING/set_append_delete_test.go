package dnsexit_live_testing

import (
	"context"
	"fmt"
	"net/netip"
	"os"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/joho/godotenv"
	"github.com/libdns/dnsexit"
	"github.com/libdns/libdns"
)

const liveTestsEnv = "LIBDNS_DNSEXIT_RUN_LIVE_TESTS"

var recsToAppend = []libdns.Record{
	libdns.Address{
		Name: "ipv4-append-test",
		IP:   netip.MustParseAddr("2.4.6.8"),
		TTL:  32 * time.Second,
	},
	libdns.Address{
		Name: "ipv6-append-test",
		IP:   netip.MustParseAddr("2002:db8::1"),
		TTL:  42 * time.Second,
	},
	libdns.CNAME{
		Name:   "alias-append-test",
		Target: "target2.example.com.",
		TTL:    52 * time.Second,
	},
	libdns.MX{
		Name:       "mx-append-test",
		Preference: 20,
		Target:     "mail2.example.com.",
		TTL:        62 * time.Second,
	},
	libdns.TXT{
		Name: "txt-append-test",
		Text: "example2 text",
		TTL:  72 * time.Second,
	},
}
var recsToSet = []libdns.Record{
	libdns.Address{
		Name: "ipv4-set-test",
		IP:   netip.MustParseAddr("1.2.3.4"),
		TTL:  30 * time.Second,
	},
	libdns.Address{
		Name: "ipv6-set-test",
		IP:   netip.MustParseAddr("2001:db8::1"),
		TTL:  40 * time.Second,
	},
	libdns.CNAME{
		Name:   "alias-set-test",
		Target: "target.example.com.",
		TTL:    50 * time.Second,
	},
	libdns.MX{
		Name:       "mx-set-test",
		Preference: 10,
		Target:     "mail.example.com.",
		TTL:        60 * time.Second,
	},
	libdns.TXT{
		Name: "txt-set-test",
		Text: "example text",
		TTL:  70 * time.Second,
	},
}

func setup(t *testing.T) (string, *dnsexit.Provider, context.Context) {
	t.Helper()

	if os.Getenv(liveTestsEnv) != "1" {
		t.Skipf("live API tests are disabled by default; set %s=1 to run", liveTestsEnv)
	}

	err := godotenv.Load("../.env")
	if err != nil {
		t.Fatalf("error loading .env file: %v", err)
	}
	apiKey := os.Getenv("LIBDNS_DNSEXIT_API_KEY")
	if apiKey == "" {
		t.Fatal("please set the LIBDNS_DNSEXIT_API_KEY environment variable")
	}
	zone := os.Getenv("LIBDNS_DNSEXIT_ZONE")
	if zone == "" {
		t.Fatal("please set the LIBDNS_DNSEXIT_ZONE environment variable (e.g. example.com.)")
	}

	provider := &dnsexit.Provider{
		APIKey:      apiKey,
		RestyClient: resty.New(),
	}

	ctx := context.Background()
	return zone, provider, ctx
}

func TestSetRecords(t *testing.T) {
	zone, provider, ctx := setup(t)

	fmt.Println("Setting records...")
	set, err := provider.SetRecords(ctx, zone, recsToSet)
	if err != nil {
		t.Fatalf("SetRecords error: %v", err)
	}
	fmt.Printf("Set: %+v\n", set)
}

func ensureNoAppendRecords(t *testing.T, zone string, provider *dnsexit.Provider, ctx context.Context) {
	t.Helper()

	fmt.Println("Cleaning up existing append records...")
	_, err := provider.DeleteRecords(ctx, zone, recsToAppend)
	if err != nil {
		t.Logf("DeleteRecords cleanup ignored error: %v", err)
	}
}

func TestAppendRecords(t *testing.T) {
	zone, provider, ctx := setup(t)

	ensureNoAppendRecords(t, zone, provider, ctx)

	fmt.Println("Appending records...")
	added, err := provider.AppendRecords(ctx, zone, recsToAppend)
	if err != nil {
		t.Fatalf("AppendRecords error: %v", err)
	}
	fmt.Printf("Added: %+v\n", added)
}

func TestDeleteRecords(t *testing.T) {
	zone, provider, ctx := setup(t)

	fmt.Println("Preparing records for delete test...")
	_, err := provider.SetRecords(ctx, zone, recsToAppend)
	if err != nil {
		t.Fatalf("SetRecords prep error: %v", err)
	}
	_, err = provider.SetRecords(ctx, zone, recsToSet)
	if err != nil {
		t.Fatalf("SetRecords prep error: %v", err)
	}

	records := append(recsToAppend, recsToSet...)
	fmt.Println("Deleting records...")
	for _, recToDelete := range records {
		fmt.Println("Deleting record:", recToDelete)
		_, err := provider.DeleteRecords(ctx, zone, []libdns.Record{recToDelete})
		if err != nil {
			t.Fatalf("DeleteRecords error: %v", err)
		}
	}
	fmt.Printf("Deleted:%+v\n", records)
}
