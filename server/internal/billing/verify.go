// Package billing keeps who has paid for Shinar: Apple's signed records of
// each purchase, checked here with the standard library, and what they allow
// (milestone 21, ADR 0034).
package billing

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/x509"
	_ "embed"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"slices"
	"strings"
	"time"
)

// appleRootG3 is Apple Root CA - G3, from apple.com/certificateauthority. Every
// record the App Store signs chains to it. Its SHA-256 is pinned in a test.
//
//go:embed AppleRootCA-G3.cer
var appleRootG3 []byte

// AppleRoots is a pool holding only Apple Root CA - G3.
func AppleRoots() *x509.CertPool {
	cert, err := x509.ParseCertificate(appleRootG3)
	if err != nil {
		panic("billing: the embedded Apple root does not parse: " + err.Error()) // a broken build, not a runtime state
	}
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	return pool
}

var (
	// ErrInvalid means a record is not one Apple signed: a broken signature,
	// a chain that does not reach Apple's root, or not a record at all.
	ErrInvalid = errors.New("billing: not a record signed by Apple")
	// ErrWrongApp means a genuine record, for another app.
	ErrWrongApp = errors.New("billing: a record for another app")
)

// Apple's environments, as its records name them. App Review and TestFlight
// buy in the sandbox, at no cost.
const (
	EnvProduction = "Production"
	EnvSandbox    = "Sandbox"
)

// The extensions Apple's own libraries look for: the leaf certificate is the
// App Store's receipt signer, and the intermediate is Apple's Worldwide
// Developer Relations authority. A certificate Apple issued for anything else
// does not carry them.
var (
	oidAppStoreLeaf     = asn1.ObjectIdentifier{1, 2, 840, 113635, 100, 6, 11, 1}
	oidWWDRIntermediate = asn1.ObjectIdentifier{1, 2, 840, 113635, 100, 6, 2, 1}
)

// Verifier checks the records the App Store signs: transactions, renewal
// information and server notifications. Each is a JWS (ES256) carrying its
// certificate chain in its header.
type Verifier struct {
	// Roots is AppleRoots() on a server; tests make a chain of their own.
	Roots *x509.CertPool
	// BundleID is the app's: a record for another app is refused.
	BundleID string
}

// Millis is a time as Apple writes it, in milliseconds since 1970.
type Millis int64

// Time is the zero time for a date Apple left out.
func (m Millis) Time() time.Time {
	if m == 0 {
		return time.Time{}
	}
	return time.UnixMilli(int64(m)).UTC()
}

// Transaction is one purchase or renewal of the subscription
// (JWSTransactionDecodedPayload), with the fields Shinar reads.
type Transaction struct {
	TransactionID         string `json:"transactionId"`
	OriginalTransactionID string `json:"originalTransactionId"`
	BundleID              string `json:"bundleId"`
	ProductID             string `json:"productId"`
	PurchaseDate          Millis `json:"purchaseDate"`
	ExpiresDate           Millis `json:"expiresDate"`
	// AppAccountToken is the user ID the phone gave with the purchase.
	AppAccountToken   string `json:"appAccountToken"`
	Environment       string `json:"environment"`
	OfferType         int    `json:"offerType"`         // 1 is an introductory offer
	OfferDiscountType string `json:"offerDiscountType"` // "FREE_TRIAL" for ours
	RevocationDate    Millis `json:"revocationDate"`    // refunded, or taken back by Apple
	SignedDate        Millis `json:"signedDate"`
}

// FreeTrial reports whether this period is the introductory free trial.
// Apple added offerDiscountType in 2024; with it missing, an introductory
// offer is read as the free trial, the only one Shinar has.
func (t Transaction) FreeTrial() bool {
	return t.OfferType == 1 && (t.OfferDiscountType == "FREE_TRIAL" || t.OfferDiscountType == "")
}

// RenewalInfo is what happens at the end of the period
// (JWSRenewalInfoDecodedPayload), with the fields Shinar reads.
type RenewalInfo struct {
	OriginalTransactionID  string `json:"originalTransactionId"`
	AutoRenewStatus        int    `json:"autoRenewStatus"` // 1: it will renew
	GracePeriodExpiresDate Millis `json:"gracePeriodExpiresDate"`
	IsInBillingRetryPeriod bool   `json:"isInBillingRetryPeriod"`
	Environment            string `json:"environment"`
	SignedDate             Millis `json:"signedDate"`
}

// Notification is one of the App Store's server notifications, version 2
// (ResponseBodyV2DecodedPayload).
type Notification struct {
	Type       string `json:"notificationType"`
	Subtype    string `json:"subtype"`
	UUID       string `json:"notificationUUID"`
	SignedDate Millis `json:"signedDate"`
	Data       struct {
		BundleID              string `json:"bundleId"`
		Environment           string `json:"environment"`
		SignedTransactionInfo string `json:"signedTransactionInfo"`
		SignedRenewalInfo     string `json:"signedRenewalInfo"`
	} `json:"data"`
}

// Transaction checks a signed transaction and reads it.
func (v Verifier) Transaction(jws string) (Transaction, error) {
	var t Transaction
	if err := v.verify(jws, &t); err != nil {
		return Transaction{}, err
	}
	if t.BundleID != v.BundleID {
		return Transaction{}, fmt.Errorf("%w: %q", ErrWrongApp, t.BundleID)
	}
	if t.OriginalTransactionID == "" || t.TransactionID == "" || t.ExpiresDate == 0 {
		return Transaction{}, fmt.Errorf("%w: not a subscription's transaction", ErrInvalid)
	}
	return t, nil
}

// RenewalInfo checks signed renewal information and reads it.
func (v Verifier) RenewalInfo(jws string) (RenewalInfo, error) {
	var r RenewalInfo
	if err := v.verify(jws, &r); err != nil {
		return RenewalInfo{}, err
	}
	return r, nil
}

// Notification checks a notification's signed payload and reads it. The
// transaction and renewal information inside are signed again, separately.
func (v Verifier) Notification(jws string) (Notification, error) {
	var n Notification
	if err := v.verify(jws, &n); err != nil {
		return Notification{}, err
	}
	if n.Data.BundleID != v.BundleID {
		return Notification{}, fmt.Errorf("%w: %q", ErrWrongApp, n.Data.BundleID)
	}
	return n, nil
}

// verify checks a JWS and reads its payload into into.
func (v Verifier) verify(jws string, into any) error {
	parts := strings.Split(jws, ".")
	if len(parts) != 3 {
		return fmt.Errorf("%w: not a JWS", ErrInvalid)
	}
	var header struct {
		Alg string   `json:"alg"`
		X5C []string `json:"x5c"`
	}
	if err := decodeSegment(parts[0], &header); err != nil {
		return err
	}
	// Only ES256: accepting what the header asks for would let a forger ask
	// for "none".
	if header.Alg != "ES256" || len(header.X5C) < 2 {
		return fmt.Errorf("%w: alg %q with %d certificates", ErrInvalid, header.Alg, len(header.X5C))
	}
	certs := make([]*x509.Certificate, len(header.X5C))
	for i, encoded := range header.X5C {
		der, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return fmt.Errorf("%w: certificate %d: %v", ErrInvalid, i, err)
		}
		if certs[i], err = x509.ParseCertificate(der); err != nil {
			return fmt.Errorf("%w: certificate %d: %v", ErrInvalid, i, err)
		}
	}
	leaf, intermediate := certs[0], certs[1]
	if !hasExtension(leaf, oidAppStoreLeaf) || !hasExtension(intermediate, oidWWDRIntermediate) {
		return fmt.Errorf("%w: not the App Store's certificates", ErrInvalid)
	}
	// Checked just above, so not "unhandled" if Apple marks them critical:
	// Go refuses a chain with a critical extension it does not know.
	handled(leaf, oidAppStoreLeaf)
	handled(intermediate, oidWWDRIntermediate)

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return fmt.Errorf("%w: payload: %v", ErrInvalid, err)
	}
	// The chain is checked as of the day Apple signed the record, as Apple's
	// own libraries do, so that a record does not stop verifying when the
	// certificate that signed it later expires. A forged date would fail the
	// signature below.
	var dated struct {
		SignedDate Millis `json:"signedDate"`
	}
	_ = json.Unmarshal(payload, &dated)
	when := dated.SignedDate.Time()
	if when.IsZero() {
		when = time.Now()
	}
	intermediates := x509.NewCertPool()
	intermediates.AddCert(intermediate)
	if _, err := leaf.Verify(x509.VerifyOptions{
		Roots: v.Roots, Intermediates: intermediates, CurrentTime: when,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	}); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalid, err)
	}

	key, ok := leaf.PublicKey.(*ecdsa.PublicKey)
	if !ok || key.Curve != elliptic.P256() {
		return fmt.Errorf("%w: not a P-256 key", ErrInvalid)
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(sig) != 64 {
		return fmt.Errorf("%w: signature", ErrInvalid)
	}
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	r, s := new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])
	if !ecdsa.Verify(key, sum[:], r, s) {
		return fmt.Errorf("%w: the signature does not match", ErrInvalid)
	}

	decoder := json.NewDecoder(bytes.NewReader(payload))
	if err := decoder.Decode(into); err != nil {
		return fmt.Errorf("%w: payload: %v", ErrInvalid, err)
	}
	return nil
}

func decodeSegment(segment string, into any) error {
	raw, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	if err := json.Unmarshal(raw, into); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	return nil
}

func handled(cert *x509.Certificate, oid asn1.ObjectIdentifier) {
	cert.UnhandledCriticalExtensions = slices.DeleteFunc(cert.UnhandledCriticalExtensions, oid.Equal)
}

func hasExtension(cert *x509.Certificate, oid asn1.ObjectIdentifier) bool {
	for _, ext := range cert.Extensions {
		if ext.Id.Equal(oid) {
			return true
		}
	}
	return false
}
