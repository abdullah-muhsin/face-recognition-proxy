package httpapi

import "testing"

func TestDeliveryEndpointsRequireExplicitHTTPS(t *testing.T) {
	for _, value := range []string{"http://school/events", "/events", "https://user:secret@school/events", "https://school/events#fragment", "https://"} {
		if validDeliveryEndpoint(value) {
			t.Fatalf("accepted %q", value)
		}
	}
	if !validDeliveryEndpoint("https://creative.itplus.club/api/integrations/pushsdk/events") {
		t.Fatal("rejected valid endpoint")
	}
}
