package oauth

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func TestDecodeJWT_AudStringAndArray(t *testing.T) {
	type testCase struct {
		name    string
		audJSON string
		want    audienceList
	}

	tests := []testCase{
		{
			name:    "aud as string",
			audJSON: `"my-client-id"`,
			want:    audienceList{"my-client-id"},
		},
		{
			name:    "aud as array with one element",
			audJSON: `["my-client-id"]`,
			want:    audienceList{"my-client-id"},
		},
		{
			name:    "aud as array with multiple elements",
			audJSON: `["client-a", "client-b"]`,
			want:    audienceList{"client-a", "client-b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := buildJWTPayload(t, map[string]interface{}{
				"nonce": "test-nonce",
				"exp":   9999999999,
				"aud":   json.RawMessage(tt.audJSON),
				"jti":   "test-jti",
			})

			result, err := decodeJWT(payload)
			if err != nil {
				t.Fatalf("decodeJWT() error = %v", err)
			}

			if len(result.Aud) != len(tt.want) {
				t.Fatalf("Aud length = %d, want %d", len(result.Aud), len(tt.want))
			}
			for i, v := range result.Aud {
				if v != tt.want[i] {
					t.Errorf("Aud[%d] = %q, want %q", i, v, tt.want[i])
				}
			}
		})
	}
}

func TestDecodeJWT_NewFields(t *testing.T) {
	token := buildJWTPayload(t, map[string]interface{}{
		"iss":   "https://example.com",
		"sub":   "user-123",
		"aud":   "client-id",
		"exp":   9999999999,
		"iat":   1700000000,
		"nonce": "test-nonce",
		"jti":   "test-jti",
	})

	result, err := decodeJWT(token)
	if err != nil {
		t.Fatalf("decodeJWT() error = %v", err)
	}

	if result.Iss != "https://example.com" {
		t.Errorf("Iss = %q, want %q", result.Iss, "https://example.com")
	}
	if result.Sub != "user-123" {
		t.Errorf("Sub = %q, want %q", result.Sub, "user-123")
	}
	if result.Iat != 1700000000 {
		t.Errorf("Iat = %d, want %d", result.Iat, 1700000000)
	}
}

func newTestProvider(clientId string, issuer string) *OauthProvider {
	return &OauthProvider{
		clientId: clientId,
		issuer:   issuer,
	}
}

func TestOidcValidate_IssuerMismatch(t *testing.T) {
	token := buildJWTPayload(t, map[string]interface{}{
		"iss":   "https://wrong.com",
		"sub":   "user-123",
		"aud":   "client-id",
		"exp":   float64(time.Now().Unix() + 3600),
		"iat":   float64(time.Now().Unix()),
		"nonce": "test-nonce",
	})

	err := newTestProvider("client-id", "https://correct.com").oidcValidate(token)
	if err == nil {
		t.Fatal("expected issuer mismatch error, got nil")
	}
}

func TestOidcValidate_IssuerSkippedWhenEmpty(t *testing.T) {
	token := buildJWTPayload(t, map[string]interface{}{
		"iss":   "https://any.com",
		"sub":   "user-123",
		"aud":   "client-id",
		"exp":   float64(time.Now().Unix() + 3600),
		"iat":   float64(time.Now().Unix()),
		"nonce": "test-nonce",
	})

	err := newTestProvider("client-id", "").oidcValidate(token)
	if err != nil {
		t.Fatalf("expected no error with empty issuer, got: %v", err)
	}
}

func TestOidcValidate_IatFuture(t *testing.T) {
	token := buildJWTPayload(t, map[string]interface{}{
		"iss":   "https://example.com",
		"sub":   "user-123",
		"aud":   "client-id",
		"exp":   float64(time.Now().Unix() + 3600),
		"iat":   float64(time.Now().Unix() + 120), // 120 seconds in the future (beyond 60s leeway)
		"nonce": "test-nonce",
	})

	err := newTestProvider("client-id", "https://example.com").oidcValidate(token)
	if err == nil {
		t.Fatal("expected future iat error, got nil")
	}
}

func TestOidcValidate_IatFutureWithinLeeway(t *testing.T) {
	token := buildJWTPayload(t, map[string]interface{}{
		"iss":   "https://example.com",
		"sub":   "user-123",
		"aud":   "client-id",
		"exp":   float64(time.Now().Unix() + 3600),
		"iat":   float64(time.Now().Unix() + 3), // 3 seconds in the future (within 60s leeway)
		"nonce": "test-nonce",
	})

	err := newTestProvider("client-id", "https://example.com").oidcValidate(token)
	if err != nil {
		t.Fatalf("expected no error within leeway, got: %v", err)
	}
}

func TestOidcValidate_EmptyIdToken(t *testing.T) {
	err := newTestProvider("client-id", "https://example.com").oidcValidate("")
	if err != nil {
		t.Fatalf("expected no error for empty id token, got: %v", err)
	}
}

func buildJWTPayload(t *testing.T, claims map[string]interface{}) string {
	t.Helper()

	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	encoded := base64.RawURLEncoding.EncodeToString(payloadBytes)

	return "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9." + encoded + ".fake-signature"
}
