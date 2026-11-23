package billing

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"
)

// chain is a certificate chain shaped like Apple's (a root, an intermediate
// marked as Apple's developer authority, a leaf marked as the App Store's
// signer), made for tests, and the leaf's key to sign with.
type chain struct {
	roots *x509.CertPool
	key   *ecdsa.PrivateKey
	x5c   []string
}

type chainOptions struct {
	noLeafMark, noIntermediateMark bool
	leafExpires                    time.Time
}

func newChain(t *testing.T, opts chainOptions) chain {
	t.Helper()
	newKey := func() *ecdsa.PrivateKey {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		return key
	}
	serial := int64(0)
	issue := func(name string, key *ecdsa.PrivateKey, parent *x509.Certificate, parentKey *ecdsa.PrivateKey, ca bool, notAfter time.Time, mark asn1.ObjectIdentifier) *x509.Certificate {
		serial++
		template := &x509.Certificate{
			SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: name},
			NotBefore: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), NotAfter: notAfter,
			IsCA: ca, BasicConstraintsValid: true,
		}
		if ca {
			template.KeyUsage = x509.KeyUsageCertSign
		} else {
			template.KeyUsage = x509.KeyUsageDigitalSignature
		}
		if mark != nil {
			// Critical, the stricter case: Go refuses a critical extension it
			// does not know unless the verifier handles it.
			template.ExtraExtensions = []pkix.Extension{{Id: mark, Critical: true, Value: []byte{5, 0}}}
		}
		if parent == nil {
			parent, parentKey = template, key
		}
		der, err := x509.CreateCertificate(rand.Reader, template, parent, &key.PublicKey, parentKey)
		if err != nil {
			t.Fatal(err)
		}
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			t.Fatal(err)
		}
		return cert
	}
	far := time.Date(2040, 1, 1, 0, 0, 0, 0, time.UTC)
	leafExpires := far
	if !opts.leafExpires.IsZero() {
		leafExpires = opts.leafExpires
	}
	leafMark, intermediateMark := oidAppStoreLeaf, oidWWDRIntermediate
	if opts.noLeafMark {
		leafMark = nil
	}
	if opts.noIntermediateMark {
		intermediateMark = nil
	}

	rootKey, intermediateKey, leafKey := newKey(), newKey(), newKey()
	root := issue("Test Root", rootKey, nil, nil, true, far, nil)
	intermediate := issue("Test WWDR", intermediateKey, root, rootKey, true, far, intermediateMark)
	leaf := issue("Test App Store", leafKey, intermediate, intermediateKey, false, leafExpires, leafMark)

	roots := x509.NewCertPool()
	roots.AddCert(root)
	return chain{roots: roots, key: leafKey, x5c: []string{
		base64.StdEncoding.EncodeToString(leaf.Raw),
		base64.StdEncoding.EncodeToString(intermediate.Raw),
		base64.StdEncoding.EncodeToString(root.Raw),
	}}
}

// sign makes a JWS of payload, as the App Store makes them.
func (c chain) sign(t *testing.T, payload any) string {
	t.Helper()
	return c.signWith(t, "ES256", payload)
}

func (c chain) signWith(t *testing.T, alg string, payload any) string {
	t.Helper()
	header, _ := json.Marshal(map[string]any{"alg": alg, "x5c": c.x5c})
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	input := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(body)
	sum := sha256.Sum256([]byte(input))
	r, s, err := ecdsa.Sign(rand.Reader, c.key, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	return input + "." + base64.RawURLEncoding.EncodeToString(sig)
}

const testBundle = "com.pelumi.shinar"

var signedAt = time.Date(2026, 11, 23, 9, 0, 0, 0, time.UTC)

// transaction is a free-trial transaction, as Apple writes one.
func transaction(changes ...func(map[string]any)) map[string]any {
	t := map[string]any{
		"transactionId": "2000000001", "originalTransactionId": "2000000001",
		"bundleId": testBundle, "productId": "com.pelumi.shinar.monthly",
		"purchaseDate": signedAt.UnixMilli(), "expiresDate": signedAt.Add(7 * 24 * time.Hour).UnixMilli(),
		"type": "Auto-Renewable Subscription", "environment": EnvSandbox,
		"offerType": 1, "offerDiscountType": "FREE_TRIAL", "signedDate": signedAt.UnixMilli(),
		"appAccountToken": "7E3FB20B-4CDB-47CC-936D-99D65F608138",
	}
	for _, change := range changes {
		change(t)
	}
	return t
}

func TestTheAppleRootIsPinned(t *testing.T) {
	// Apple Root CA - G3's SHA-256, as Apple lists it among its trusted roots.
	const want = "63343abfb89a6a03ebb57e9b3f5fa7be7c4f5c756f3017b3a8c488c3653e9179"
	sum := sha256.Sum256(appleRootG3)
	if got := hex.EncodeToString(sum[:]); got != want {
		t.Fatalf("embedded root SHA-256 = %s, want %s", got, want)
	}
	if AppleRoots() == nil {
		t.Fatal("no pool")
	}
}

func TestVerifyReadsATransaction(t *testing.T) {
	c := newChain(t, chainOptions{})
	v := Verifier{Roots: c.roots, BundleID: testBundle}

	got, err := v.Transaction(c.sign(t, transaction()))
	if err != nil {
		t.Fatal(err)
	}
	if got.OriginalTransactionID != "2000000001" || got.ProductID != "com.pelumi.shinar.monthly" || !got.FreeTrial() ||
		!got.ExpiresDate.Time().Equal(signedAt.Add(7*24*time.Hour)) || got.Environment != EnvSandbox {
		t.Errorf("transaction = %+v", got)
	}
}

func TestVerifyRefusesWhatAppleDidNotSign(t *testing.T) {
	good := newChain(t, chainOptions{})
	other := newChain(t, chainOptions{}) // a chain that does not reach the trusted root
	tamper := func(jws string) string {
		parts := strings.Split(jws, ".")
		body, _ := json.Marshal(transaction(func(t map[string]any) { t["expiresDate"] = signedAt.Add(400 * 24 * time.Hour).UnixMilli() }))
		return parts[0] + "." + base64.RawURLEncoding.EncodeToString(body) + "." + parts[2]
	}
	tests := []struct {
		name string
		jws  string
		want error
	}{
		{"another root", other.sign(t, transaction()), ErrInvalid},
		{"a leaf without the App Store's mark", newChain(t, chainOptions{noLeafMark: true}).sign(t, transaction()), ErrInvalid},
		{"an intermediate without Apple's mark", newChain(t, chainOptions{noIntermediateMark: true}).sign(t, transaction()), ErrInvalid},
		{"alg none", good.signWith(t, "none", transaction()), ErrInvalid},
		{"a changed payload", tamper(good.sign(t, transaction())), ErrInvalid},
		{"not a JWS", "hello", ErrInvalid},
		{"no signature", strings.Join(strings.Split(good.sign(t, transaction()), ".")[:2], ".") + ".", ErrInvalid},
		{"a leaf expired when it signed", newChain(t, chainOptions{leafExpires: signedAt.Add(-time.Hour)}).sign(t, transaction()), ErrInvalid},
		{"another app", good.sign(t, transaction(func(t map[string]any) { t["bundleId"] = "com.example.other" })), ErrWrongApp},
		{"not a subscription", good.sign(t, transaction(func(t map[string]any) { delete(t, "expiresDate") })), ErrInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			roots := good.roots
			if strings.Contains(tt.name, "mark") || strings.Contains(tt.name, "expired") {
				roots = nil // those chains are trusted below, so only the mark or the date fails them
			}
			v := Verifier{Roots: roots, BundleID: testBundle}
			if roots == nil {
				v.Roots = rootOf(t, tt.jws)
			}
			if _, err := v.Transaction(tt.jws); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

// rootOf trusts the root a test JWS carries last in its chain.
func rootOf(t *testing.T, jws string) *x509.CertPool {
	t.Helper()
	header, _ := base64.RawURLEncoding.DecodeString(strings.Split(jws, ".")[0])
	var h struct {
		X5C []string `json:"x5c"`
	}
	if err := json.Unmarshal(header, &h); err != nil {
		t.Fatal(err)
	}
	der, _ := base64.StdEncoding.DecodeString(h.X5C[len(h.X5C)-1])
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	return pool
}

func TestVerifyReadsANotificationAndItsBundle(t *testing.T) {
	c := newChain(t, chainOptions{})
	v := Verifier{Roots: c.roots, BundleID: testBundle}
	note := func(bundle string) map[string]any {
		return map[string]any{
			"notificationType": "DID_RENEW", "notificationUUID": "n-1", "signedDate": signedAt.UnixMilli(),
			"data": map[string]any{"bundleId": bundle, "environment": EnvSandbox, "signedTransactionInfo": "x"},
		}
	}
	n, err := v.Notification(c.sign(t, note(testBundle)))
	if err != nil || n.Type != "DID_RENEW" || n.UUID != "n-1" || n.Data.SignedTransactionInfo != "x" {
		t.Errorf("notification = %+v, %v", n, err)
	}
	if _, err := v.Notification(c.sign(t, note("com.example.other"))); !errors.Is(err, ErrWrongApp) {
		t.Errorf("another app's notification: err = %v, want ErrWrongApp", err)
	}
}

// TestLiveAppleSignature asks Apple's sandbox for a test notification and
// checks the payload Apple signed with the real verifier: the In-App Purchase
// key, the API token and the verifier against Apple's own certificates. It
// runs only with the key (make appstore-check).
func TestLiveAppleSignature(t *testing.T) {
	keyFile, keyID, issuer := os.Getenv("APPSTORE_KEY_FILE"), os.Getenv("APPSTORE_KEY_ID"), os.Getenv("APPSTORE_ISSUER_ID")
	if keyFile == "" || keyID == "" || issuer == "" {
		t.Skip("APPSTORE_KEY_FILE, APPSTORE_KEY_ID and APPSTORE_ISSUER_ID are not set")
	}
	p8, err := os.ReadFile(keyFile)
	if err != nil {
		t.Fatal(err)
	}
	key, err := ParseKey(p8)
	if err != nil {
		t.Fatal(err)
	}
	api := AppStoreAPI{KeyID: keyID, IssuerID: issuer, BundleID: testBundle, Key: key, Sandbox: true}
	ctx := context.Background()
	token, err := api.RequestTestNotification(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var payload string
	for attempt := 0; attempt < 10 && payload == ""; attempt++ {
		time.Sleep(2 * time.Second) // Apple makes it in the background
		payload, err = api.TestNotification(ctx, token)
	}
	if err != nil || payload == "" {
		t.Fatalf("no test notification from Apple: %v", err)
	}
	n, err := Verifier{Roots: AppleRoots(), BundleID: testBundle}.Notification(payload)
	if err != nil {
		t.Fatalf("Apple's own signature did not verify: %v", err)
	}
	t.Logf("verified Apple's %s notification %s, %s", n.Type, n.UUID, n.Data.Environment)
}
