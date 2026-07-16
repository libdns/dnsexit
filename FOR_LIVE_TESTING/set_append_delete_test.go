package dnsexit_live_testing

import (
	"context"
	"fmt"
	"net/netip"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/joho/godotenv"
	"github.com/libdns/dnsexit"
	"github.com/libdns/libdns"
)

const liveTestsEnv = "LIBDNS_DNSEXIT_RUN_LIVE_TESTS"

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func withTrailingDot(value string) string {
	trimmed := strings.TrimSpace(strings.TrimSuffix(value, "."))
	if trimmed == "" {
		return "example.com."
	}
	return trimmed + "."
}

func stripTrailingDot(value string) string {
	return strings.TrimSuffix(strings.TrimSpace(value), ".")
}

func joinRelativeLabel(label, subdomain string) string {
	if subdomain == "" {
		return label
	}
	return label + "." + subdomain
}

func fqdnForSubdomain(subdomain, zone string) string {
	baseZone := stripTrailingDot(zone)
	if subdomain == "" {
		return baseZone + "."
	}
	return subdomain + "." + baseZone + "."
}

func buildRecords(prefix, subdomain, zone string) ([]libdns.Record, []libdns.Record) {
	targetBase := fqdnForSubdomain(subdomain, zone)

	setRecords := []libdns.Record{
		libdns.Address{
			Name: joinRelativeLabel(prefix+"-ipv4-set", subdomain),
			IP:   netip.MustParseAddr("1.2.3.4"),
			TTL:  30 * time.Second,
		},
		libdns.Address{
			Name: joinRelativeLabel(prefix+"-ipv6-set", subdomain),
			IP:   netip.MustParseAddr("2001:db8::1"),
			TTL:  40 * time.Second,
		},
		libdns.CNAME{
			Name:   joinRelativeLabel(prefix+"-alias-set", subdomain),
			Target: "target." + targetBase,
			TTL:    50 * time.Second,
		},
		libdns.MX{
			Name:       joinRelativeLabel(prefix+"-mx-set", subdomain),
			Preference: 10,
			Target:     "mail." + targetBase,
			TTL:        60 * time.Second,
		},
		libdns.TXT{
			Name: joinRelativeLabel(prefix+"-txt-set", subdomain),
			Text: "example text",
			TTL:  70 * time.Second,
		},
	}

	appendRecords := []libdns.Record{
		libdns.Address{
			Name: joinRelativeLabel(prefix+"-ipv4-append", subdomain),
			IP:   netip.MustParseAddr("2.4.6.8"),
			TTL:  32 * time.Second,
		},
		libdns.Address{
			Name: joinRelativeLabel(prefix+"-ipv6-append", subdomain),
			IP:   netip.MustParseAddr("2002:db8::1"),
			TTL:  42 * time.Second,
		},
		libdns.CNAME{
			Name:   joinRelativeLabel(prefix+"-alias-append", subdomain),
			Target: "target2." + targetBase,
			TTL:    52 * time.Second,
		},
		libdns.MX{
			Name:       joinRelativeLabel(prefix+"-mx-append", subdomain),
			Preference: 20,
			Target:     "mail2." + targetBase,
			TTL:        62 * time.Second,
		},
		libdns.TXT{
			Name: joinRelativeLabel(prefix+"-txt-append", subdomain),
			Text: "example2 text",
			TTL:  72 * time.Second,
		},
	}

	return setRecords, appendRecords
}

func setup(t *testing.T) (string, *dnsexit.Provider, context.Context, []libdns.Record, []libdns.Record) {
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
	zone = withTrailingDot(zone)
	prefix := envOrDefault("LIBDNS_DNSEXIT_TEST_RECORD_PREFIX", "libdns-live")
	subdomain := strings.Trim(strings.TrimSpace(envOrDefault("LIBDNS_DNSEXIT_TEST_SUBDOMAIN", "test_subdomain")), ".")
	recsToSet, recsToAppend := buildRecords(prefix, subdomain, zone)

	provider := &dnsexit.Provider{
		APIKey:      apiKey,
		RestyClient: resty.New(),
	}

	ctx := context.Background()
	return zone, provider, ctx, recsToSet, recsToAppend
}

func TestSetRecords(t *testing.T) {
	zone, provider, ctx, recsToSet, _ := setup(t)

	fmt.Println("Setting records...")
	set, err := provider.SetRecords(ctx, zone, recsToSet)
	if err != nil {
		t.Fatalf("SetRecords error: %v", err)
	}
	fmt.Printf("Set: %+v\n", set)
}

func ensureNoAppendRecords(t *testing.T, zone string, provider *dnsexit.Provider, ctx context.Context, recsToAppend []libdns.Record) {
	t.Helper()

	fmt.Println("Cleaning up existing append records...")
	_, err := provider.DeleteRecords(ctx, zone, recsToAppend)
	if err != nil {
		t.Logf("DeleteRecords cleanup ignored error: %v", err)
	}
}

func TestAppendRecords(t *testing.T) {
	zone, provider, ctx, _, recsToAppend := setup(t)

	ensureNoAppendRecords(t, zone, provider, ctx, recsToAppend)

	fmt.Println("Appending records...")
	added, err := provider.AppendRecords(ctx, zone, recsToAppend)
	if err != nil {
		t.Fatalf("AppendRecords error: %v", err)
	}
	fmt.Printf("Added: %+v\n", added)
}

func TestDeleteRecords(t *testing.T) {
	zone, provider, ctx, recsToSet, recsToAppend := setup(t)

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
