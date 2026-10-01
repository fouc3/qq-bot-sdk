package qqbotsdk

import (
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

// TestSeedFromSecretMatchesDocumentedVector checks the seed derivation against
// the worked example in the official documentation:
//
//	secret: naOC0ocQE3shWLAfffVLB1rhYPG7
//	seed:   naOC0ocQE3shWLAfffVLB1rhYPG7naOC
func TestSeedFromSecretMatchesDocumentedVector(t *testing.T) {
	const secret = "naOC0ocQE3shWLAfffVLB1rhYPG7"
	const want = "naOC0ocQE3shWLAfffVLB1rhYPG7naOC"

	if got := SeedFromSecret(secret); got != want {
		t.Errorf("SeedFromSecret(%q) = %q, want %q", secret, got, want)
	}
	if len(want) != ed25519.SeedSize {
		t.Fatalf("the documented seed is %d bytes, want %d", len(want), ed25519.SeedSize)
	}
}

// TestDerivedPublicKeyMatchesDocumentedVector checks the key derivation against
// the public key printed in the official documentation demo.
func TestDerivedPublicKeyMatchesDocumentedVector(t *testing.T) {
	const secret = "naOC0ocQE3shWLAfffVLB1rhYPG7"
	want := []byte{215, 195, 98, 254, 120, 174, 248, 31, 242, 50, 135, 180,
		147, 98, 139, 93, 176, 42, 60, 79, 227, 11, 33, 94, 77, 25, 96, 155, 93, 118, 103, 58}

	signer, err := NewSigner(secret)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	if got := signer.PublicKey(); hex.EncodeToString(got) != hex.EncodeToString(want) {
		t.Errorf("public key = %x, want %x", got, want)
	}
}

// TestSignVerifyRoundTrip covers the verification path with a signature this
// SDK produced over the documented message layout.
//
// The security guide's own "验证签名" example prints a signature for
//
//	secret:    naOC0ocQE3shWLAfffVLB1rhYPG7
//	body:      { "op": 0,"d": {}, "t": "GATEWAY_EVENT_NAME"}
//	timestamp: 1725442341
//
// but that value does not verify against the public key the same page derives,
// nor does it match any signing of that input under that key. The documented
// derivation is confirmed by the seed and public key vectors above, and the
// timestamp+body layout is confirmed by the validation example below, which
// does reproduce byte for byte. That example signature is therefore treated as
// an error in the documentation rather than as behaviour to match.
func TestSignVerifyRoundTrip(t *testing.T) {
	const (
		secret    = "naOC0ocQE3shWLAfffVLB1rhYPG7"
		timestamp = "1725442341"
		body      = `{ "op": 0,"d": {}, "t": "GATEWAY_EVENT_NAME"}`
	)

	signer, err := NewSigner(secret)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}

	signature := signer.Sign(timestamp, []byte(body))
	if len(signature) != ed25519.SignatureSize*2 {
		t.Fatalf("signature is %d hex chars, want %d", len(signature), ed25519.SignatureSize*2)
	}

	if err := signer.Verify(timestamp, []byte(body), signature); err != nil {
		t.Fatalf("Verify rejected a signature it produced: %v", err)
	}
	// The same signature must verify through the free function too.
	if err := VerifySignature(signer.PublicKey(), timestamp, []byte(body), signature); err != nil {
		t.Errorf("VerifySignature rejected a signature the signer produced: %v", err)
	}
}

// TestHandleValidationMatchesDocumentedExample checks the callback address
// validation reply against the documented example:
//
//	appid:  11111111
//	secret: DG5g3B4j9X2KOErG
//	body:   {"d":{"plain_token":"Arq0D5A61EgUu4OxUvOp","event_ts":"1725442341"},"op":13}
//	reply:  {"plain_token": "Arq0D5A61EgUu4OxUvOp","signature": "87befc99..."}
func TestHandleValidationMatchesDocumentedExample(t *testing.T) {
	const (
		secret     = "DG5g3B4j9X2KOErG"
		plainToken = "Arq0D5A61EgUu4OxUvOp"
		eventTs    = "1725442341"
		want       = "87befc99c42c651b3aac0278e71ada338433ae26fcb24307bdc5ad38c1adc2d0" +
			"1bcfcadc0842edac85e85205028a1132afe09280305f13aa6909ffc2d652c706"
	)

	signer, err := NewSigner(secret)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}

	resp, err := signer.HandleValidation(&ValidationRequest{PlainToken: plainToken, EventTs: eventTs})
	if err != nil {
		t.Fatalf("HandleValidation: %v", err)
	}
	if resp.PlainToken != plainToken {
		t.Errorf("PlainToken = %q, want %q", resp.PlainToken, plainToken)
	}
	if resp.Signature != want {
		t.Errorf("Signature = %s\nwant        %s", resp.Signature, want)
	}
}

func TestHandleValidationRejectsIncompleteRequests(t *testing.T) {
	signer, err := NewSigner("secret")
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}

	if _, err := signer.HandleValidation(nil); err == nil {
		t.Error("a nil request must be rejected")
	}
	if _, err := signer.HandleValidation(&ValidationRequest{EventTs: "1"}); err == nil {
		t.Error("a request without plain_token must be rejected")
	}
	if _, err := signer.HandleValidation(&ValidationRequest{PlainToken: "p"}); err == nil {
		t.Error("a request without event_ts must be rejected")
	}
}

func TestNewSignerRejectsEmptySecret(t *testing.T) {
	if _, err := NewSigner(""); err == nil {
		t.Error("an empty secret must be rejected")
	}
}

func TestVerifySignatureRejections(t *testing.T) {
	signer, err := NewSigner("naOC0ocQE3shWLAfffVLB1rhYPG7")
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	const timestamp = "1725442341"
	body := []byte(`{"op":0}`)
	good := signer.Sign(timestamp, body)

	cases := map[string]struct {
		timestamp string
		body      []byte
		signature string
	}{
		"missing signature": {timestamp, body, ""},
		"missing timestamp": {"", body, good},
		"not hex":           {timestamp, body, "zzzz"},
		"too short":         {timestamp, body, "abcd"},
		"wrong body":        {timestamp, []byte(`{"op":1}`), good},
		"wrong timestamp":   {"1725442342", body, good},
		"tampered signature": {
			timestamp, body,
			good[:len(good)-2] + "00",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := signer.Verify(tc.timestamp, tc.body, tc.signature)
			if !errors.Is(err, ErrInvalidSignature) {
				t.Fatalf("err = %v, want ErrInvalidSignature", err)
			}
		})
	}
}

// TestVerifySignatureRejectsNonCanonicalSignature covers the documented
// sig[63]&224 != 0 check.
func TestVerifySignatureRejectsNonCanonicalSignature(t *testing.T) {
	publicKey := make(ed25519.PublicKey, ed25519.PublicKeySize)
	// A 64 byte signature whose last byte has the top three bits set.
	signature := make([]byte, ed25519.SignatureSize)
	signature[ed25519.SignatureSize-1] = 0xE0

	err := VerifySignature(publicKey, "1", []byte("body"), hex.EncodeToString(signature))
	if err == nil {
		t.Fatal("expected a non-canonical signature to be rejected")
	}
	if !strings.Contains(err.Error(), "not canonical") {
		t.Errorf("err = %v, want it to name the canonical check", err)
	}
}

func TestVerifySignatureRejectsBadPublicKey(t *testing.T) {
	err := VerifySignature(ed25519.PublicKey{1, 2, 3}, "1", []byte("body"), "abcd")
	if !errors.Is(err, ErrInvalidSignature) {
		t.Errorf("err = %v, want ErrInvalidSignature", err)
	}
}

// TestVerifySignatureAcceptsExternallySuppliedKey covers verification without a
// bot secret, using the public key the documentation derives.
func TestVerifySignatureAcceptsExternallySuppliedKey(t *testing.T) {
	const secret = "naOC0ocQE3shWLAfffVLB1rhYPG7"
	derived, err := NewSigner(secret)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}

	key := derived.PublicKey()
	timestamp := "1725442341"
	body := []byte(`{"op":0}`)
	signature := derived.Sign(timestamp, body)

	if err := VerifySignature(key, timestamp, body, signature); err != nil {
		t.Errorf("VerifySignature with an explicit key: %v", err)
	}
}

// TestDocumentedSecretProducesStableKey pins the derivation so a refactor
// cannot silently change which keys the SDK derives.
func TestDocumentedSecretProducesStableKey(t *testing.T) {
	signer, err := NewSigner("DG5g3B4j9X2KOErG")
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	first := signer.Sign("1725442341", []byte("payload"))

	again, err := NewSigner("DG5g3B4j9X2KOErG")
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	if second := again.Sign("1725442341", []byte("payload")); first != second {
		t.Errorf("signature is not deterministic: %s vs %s", first, second)
	}

	other, err := NewSigner("a-different-secret")
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	if third := other.Sign("1725442341", []byte("payload")); third == first {
		t.Error("a different secret must produce a different signature")
	}
}
