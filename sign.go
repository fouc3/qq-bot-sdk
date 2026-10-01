package qqbotsdk

import (
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// Webhook signature headers, as documented.
const (
	// SignatureHeader carries the hex encoded Ed25519 signature.
	SignatureHeader = "X-Signature-Ed25519"
	// SignatureTimestampHeader carries the timestamp used in the signature.
	SignatureTimestampHeader = "X-Signature-Timestamp"
	// BotAppIDHeader carries the AppID on a callback request.
	BotAppIDHeader = "X-Bot-Appid"
	// CallbackUserAgent is the User-Agent the platform sends on callbacks.
	CallbackUserAgent = "QQBot-Callback"
)

// signatureInvalidBitmask rejects signatures whose top three bits are set.
//
// The documented check is sig[63]&224 != 0, which guards against
// non-canonical signatures.
const signatureInvalidBitmask = 0xE0

// ErrInvalidSignature reports a callback whose signature did not verify.
var ErrInvalidSignature = errors.New("qqbotsdk: invalid callback signature")

// Signer verifies callback signatures and answers the callback address
// validation, using the bot secret.
//
// The documented algorithm derives the Ed25519 key pair deterministically from
// the bot secret: the secret is repeated until it fills the 32 byte seed.
type Signer struct {
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
}

// NewSigner derives a Signer from the bot secret.
//
// The secret is repeated until it reaches the Ed25519 seed size, exactly as
// the documentation describes, so the derived keys match the platform's.
func NewSigner(botSecret string) (*Signer, error) {
	if botSecret == "" {
		return nil, errors.New("qqbotsdk: empty bot secret")
	}
	seed := SeedFromSecret(botSecret)
	privateKey := ed25519.NewKeyFromSeed([]byte(seed))
	return &Signer{
		privateKey: privateKey,
		publicKey:  privateKey.Public().(ed25519.PublicKey),
	}, nil
}

// SeedFromSecret repeats the secret until it fills an Ed25519 seed and returns
// those 32 bytes.
//
// It is exported because it is the one piece of the scheme that callers
// implementing their own verification need to reproduce exactly.
func SeedFromSecret(botSecret string) string {
	seed := botSecret
	for len(seed) < ed25519.SeedSize {
		seed = strings.Repeat(seed, 2)
	}
	return seed[:ed25519.SeedSize]
}

// PublicKey returns the derived public key.
func (s *Signer) PublicKey() ed25519.PublicKey {
	return s.publicKey
}

// Verify checks a callback signature.
//
// The signed message is the timestamp followed by the raw request body, and
// the signature is hex encoded in the X-Signature-Ed25519 header.
func (s *Signer) Verify(timestamp string, body []byte, signatureHex string) error {
	return VerifySignature(s.publicKey, timestamp, body, signatureHex)
}

// VerifySignature checks a signature against a public key.
//
// It is a free function so a caller can verify without deriving a Signer, for
// example when the public key is managed elsewhere.
func VerifySignature(publicKey ed25519.PublicKey, timestamp string, body []byte, signatureHex string) error {
	if signatureHex == "" {
		return fmt.Errorf("%w: missing %s header", ErrInvalidSignature, SignatureHeader)
	}
	if timestamp == "" {
		return fmt.Errorf("%w: missing %s header", ErrInvalidSignature, SignatureTimestampHeader)
	}
	if len(publicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("%w: unexpected public key size %d", ErrInvalidSignature, len(publicKey))
	}

	signature, err := hex.DecodeString(signatureHex)
	if err != nil {
		return fmt.Errorf("%w: signature is not valid hex: %v", ErrInvalidSignature, err)
	}
	// The documentation rejects a signature of the wrong length, and one whose
	// final byte has the high three bits set.
	if len(signature) != ed25519.SignatureSize {
		return fmt.Errorf("%w: signature is %d bytes, want %d", ErrInvalidSignature, len(signature), ed25519.SignatureSize)
	}
	if signature[len(signature)-1]&signatureInvalidBitmask != 0 {
		return fmt.Errorf("%w: signature is not canonical", ErrInvalidSignature)
	}

	if !ed25519.Verify(publicKey, signedMessage(timestamp, body), signature) {
		return ErrInvalidSignature
	}
	return nil
}

// Sign returns the hex encoded signature of the timestamp and body pair.
func (s *Signer) Sign(timestamp string, body []byte) string {
	return hex.EncodeToString(ed25519.Sign(s.privateKey, signedMessage(timestamp, body)))
}

// signedMessage builds the signed payload: timestamp followed by body.
//
// The documentation is explicit that the order is timestamp + body, with no
// separator.
func signedMessage(timestamp string, body []byte) []byte {
	msg := make([]byte, 0, len(timestamp)+len(body))
	msg = append(msg, timestamp...)
	msg = append(msg, body...)
	return msg
}

// ValidationRequest is the payload.d of a callback address validation request
// (op 13).
type ValidationRequest struct {
	// PlainToken is the string to sign.
	PlainToken string `json:"plain_token"`
	// EventTs is the timestamp used when signing.
	EventTs string `json:"event_ts"`
}

// ValidationResponse is the body a bot must return to a validation request.
type ValidationResponse struct {
	// PlainToken echoes the request value.
	PlainToken string `json:"plain_token"`
	// Signature is the hex encoded signature of event_ts followed by
	// plain_token.
	Signature string `json:"signature"`
}

// HandleValidation answers a callback address validation request.
//
// The signed message is event_ts followed by plain_token, and the reply echoes
// the plain token with the signature.
func (s *Signer) HandleValidation(req *ValidationRequest) (*ValidationResponse, error) {
	if req == nil || req.PlainToken == "" {
		return nil, errors.New("qqbotsdk: validation request has no plain_token")
	}
	if req.EventTs == "" {
		return nil, errors.New("qqbotsdk: validation request has no event_ts")
	}
	// The validation reply signs event_ts + plain_token. Reusing Sign keeps the
	// concatenation rule in one place.
	signature := s.Sign(req.EventTs, []byte(req.PlainToken))
	return &ValidationResponse{
		PlainToken: req.PlainToken,
		Signature:  signature,
	}, nil
}
