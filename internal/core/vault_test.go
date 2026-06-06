package core_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/rpatton4/electrorangerd/internal/core"
	"github.com/rpatton4/electrorangerd/internal/domain"
	"github.com/rpatton4/electrorangerd/internal/errs"
)

// memStore is an in-memory EncryptedStore for tests. It mimics the on-disk
// adapter's "not found" signal via errs.ErrNotFound.
type memStore struct {
	blob   domain.VaultBlob
	hasIt  bool
	saveCt int
}

func (m *memStore) Load(_ context.Context) (domain.VaultBlob, error) {
	if !m.hasIt {
		return domain.VaultBlob{}, errs.ErrNotFound
	}
	return m.blob, nil
}

func (m *memStore) Save(_ context.Context, blob domain.VaultBlob) error {
	m.blob = blob
	m.hasIt = true
	m.saveCt++
	return nil
}

func (m *memStore) Delete(_ context.Context) error {
	m.blob = domain.VaultBlob{}
	m.hasIt = false
	return nil
}

// memKeychain is an in-memory Keychain for tests.
type memKeychain struct {
	available bool
	dek       []byte
	has       bool
}

func (k *memKeychain) Available() bool { return k.available }

func (k *memKeychain) Set(_ context.Context, dek []byte) error {
	if !k.available {
		return errs.ErrKeychainUnavailable
	}
	k.dek = append([]byte(nil), dek...)
	k.has = true
	return nil
}

func (k *memKeychain) Get(_ context.Context) ([]byte, error) {
	if !k.available {
		return nil, errs.ErrKeychainUnavailable
	}
	if !k.has {
		return nil, errs.ErrNotFound
	}
	return append([]byte(nil), k.dek...), nil
}

func (k *memKeychain) Delete(_ context.Context) error {
	if !k.available {
		return errs.ErrKeychainUnavailable
	}
	if !k.has {
		return errs.ErrNotFound
	}
	k.dek = nil
	k.has = false
	return nil
}

func TestVaultLifecycle(t *testing.T) {
	store := &memStore{}
	kc := &memKeychain{available: true}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	v, err := core.NewVault(ctx, store, kc, logger)
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	if v.Status() != domain.VaultStatusUninitialized {
		t.Fatalf("fresh vault: want Uninitialized, got %v", v.Status())
	}

	const pw1 = "correct horse battery staple"
	if err := v.Initialize(ctx, pw1); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if v.Status() != domain.VaultStatusUnlocked {
		t.Fatalf("after Initialize: want Unlocked, got %v", v.Status())
	}

	if err := v.Initialize(ctx, pw1); !errors.Is(err, errs.ErrVaultAlreadyInitialized) {
		t.Fatalf("second Initialize: want ErrVaultAlreadyInitialized, got %v", err)
	}

	const profileName = "production"
	const dsn = "postgres://user:secret@db.example.com:5432/app?sslmode=require"
	if err := v.SaveProfileDSN(ctx, profileName, dsn); err != nil {
		t.Fatalf("SaveProfileDSN: %v", err)
	}

	got, err := v.GetProfileDSN(profileName)
	if err != nil {
		t.Fatalf("GetProfileDSN: %v", err)
	}
	if got != dsn {
		t.Fatalf("GetProfileDSN: round-trip mismatch")
	}

	names, err := v.ListProfiles()
	if err != nil {
		t.Fatalf("ListProfiles: %v", err)
	}
	if len(names) != 1 || names[0] != profileName {
		t.Fatalf("ListProfiles: want [%q], got %v", profileName, names)
	}

	v.Lock()
	if v.Status() != domain.VaultStatusLocked {
		t.Fatalf("after Lock: want Locked, got %v", v.Status())
	}

	if _, err := v.GetProfileDSN(profileName); !errors.Is(err, errs.ErrVaultLocked) {
		t.Fatalf("GetProfileDSN when locked: want ErrVaultLocked, got %v", err)
	}
	if err := v.SaveProfileDSN(ctx, "x", "y"); !errors.Is(err, errs.ErrVaultLocked) {
		t.Fatalf("SaveProfileDSN when locked: want ErrVaultLocked, got %v", err)
	}

	if err := v.Unlock(ctx, "wrong password"); !errors.Is(err, errs.ErrInvalidPassword) {
		t.Fatalf("Unlock with wrong password: want ErrInvalidPassword, got %v", err)
	}
	if v.Status() != domain.VaultStatusLocked {
		t.Fatalf("after failed Unlock: want still Locked, got %v", v.Status())
	}

	if err := v.Unlock(ctx, pw1); err != nil {
		t.Fatalf("Unlock with correct password: %v", err)
	}
	got, err = v.GetProfileDSN(profileName)
	if err != nil || got != dsn {
		t.Fatalf("GetProfileDSN after re-Unlock: got=%q err=%v", got, err)
	}
}

func TestVaultRestartFromDisk(t *testing.T) {
	store := &memStore{}
	kc := &memKeychain{available: true}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	v1, err := core.NewVault(ctx, store, kc, logger)
	if err != nil {
		t.Fatalf("NewVault #1: %v", err)
	}
	const pw = "hunter2-hunter2"
	if err := v1.Initialize(ctx, pw); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if err := v1.SaveProfileDSN(ctx, "staging", "postgres://x"); err != nil {
		t.Fatalf("SaveProfileDSN: %v", err)
	}

	// Simulate a fresh process start by constructing a new vault against the
	// same store. The keychain has not been enabled.
	v2, err := core.NewVault(ctx, store, kc, logger)
	if err != nil {
		t.Fatalf("NewVault #2: %v", err)
	}
	if v2.Status() != domain.VaultStatusLocked {
		t.Fatalf("new instance with existing blob: want Locked, got %v", v2.Status())
	}
	if err := v2.Unlock(ctx, pw); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	if dsn, err := v2.GetProfileDSN("staging"); err != nil || dsn != "postgres://x" {
		t.Fatalf("GetProfileDSN after restart: dsn=%q err=%v", dsn, err)
	}
}

func TestVaultChangePasswordPreservesProfiles(t *testing.T) {
	store := &memStore{}
	kc := &memKeychain{available: true}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	v, err := core.NewVault(ctx, store, kc, logger)
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	const pwOld = "old-pass"
	const pwNew = "new-pass"

	if err := v.Initialize(ctx, pwOld); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if err := v.SaveProfileDSN(ctx, "p", "postgres://abc"); err != nil {
		t.Fatalf("SaveProfileDSN: %v", err)
	}

	if err := v.ChangePassword(ctx, "wrong-old", pwNew); !errors.Is(err, errs.ErrInvalidPassword) {
		t.Fatalf("ChangePassword with wrong old: want ErrInvalidPassword, got %v", err)
	}
	if err := v.ChangePassword(ctx, pwOld, pwNew); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}

	// Profile is still readable with the same in-memory DEK.
	if dsn, err := v.GetProfileDSN("p"); err != nil || dsn != "postgres://abc" {
		t.Fatalf("GetProfileDSN after ChangePassword: dsn=%q err=%v", dsn, err)
	}

	// Lock, then unlock with the new password.
	v.Lock()
	if err := v.Unlock(ctx, pwOld); !errors.Is(err, errs.ErrInvalidPassword) {
		t.Fatalf("Unlock with old password after change: want ErrInvalidPassword, got %v", err)
	}
	if err := v.Unlock(ctx, pwNew); err != nil {
		t.Fatalf("Unlock with new password: %v", err)
	}
	if dsn, err := v.GetProfileDSN("p"); err != nil || dsn != "postgres://abc" {
		t.Fatalf("GetProfileDSN after re-Unlock with new password: dsn=%q err=%v", dsn, err)
	}
}

func TestVaultProfileCRUD(t *testing.T) {
	store := &memStore{}
	kc := &memKeychain{available: true}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	v, err := core.NewVault(ctx, store, kc, logger)
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	if err := v.Initialize(ctx, "pw"); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	tests := []struct {
		op   string
		do   func() error
		want []string
	}{
		{"save a", func() error { return v.SaveProfileDSN(ctx, "a", "dsn-a") }, []string{"a"}},
		{"save b", func() error { return v.SaveProfileDSN(ctx, "b", "dsn-b") }, []string{"a", "b"}},
		{"upsert a", func() error { return v.SaveProfileDSN(ctx, "a", "dsn-a-2") }, []string{"a", "b"}},
		{"delete b", func() error { return v.DeleteProfile(ctx, "b") }, []string{"a"}},
	}
	for _, tc := range tests {
		if err := tc.do(); err != nil {
			t.Fatalf("%s: %v", tc.op, err)
		}
		got, err := v.ListProfiles()
		if err != nil {
			t.Fatalf("ListProfiles after %s: %v", tc.op, err)
		}
		if !stringSliceEqual(got, tc.want) {
			t.Fatalf("after %s: want %v, got %v", tc.op, tc.want, got)
		}
	}

	if dsn, err := v.GetProfileDSN("a"); err != nil || dsn != "dsn-a-2" {
		t.Fatalf("upsert did not overwrite: dsn=%q err=%v", dsn, err)
	}
	if err := v.DeleteProfile(ctx, "nonexistent"); !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("DeleteProfile missing: want ErrNotFound, got %v", err)
	}
}

func TestVaultKeychainRoundTrip(t *testing.T) {
	store := &memStore{}
	kc := &memKeychain{available: true}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	v, err := core.NewVault(ctx, store, kc, logger)
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	if err := v.Initialize(ctx, "pw"); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if err := v.SaveProfileDSN(ctx, "p", "dsn"); err != nil {
		t.Fatalf("SaveProfileDSN: %v", err)
	}

	if v.IsKeychainEnabled() {
		t.Fatalf("IsKeychainEnabled before EnableKeychain: want false")
	}
	if err := v.EnableKeychain(ctx); err != nil {
		t.Fatalf("EnableKeychain: %v", err)
	}
	if !v.IsKeychainEnabled() {
		t.Fatalf("IsKeychainEnabled after EnableKeychain: want true")
	}

	// Simulate restart: new vault instance, keychain still has the DEK.
	v2, err := core.NewVault(ctx, store, kc, logger)
	if err != nil {
		t.Fatalf("NewVault #2: %v", err)
	}
	if v2.Status() != domain.VaultStatusLocked {
		t.Fatalf("restart: want Locked, got %v", v2.Status())
	}
	if err := v2.TryKeychainUnlock(ctx); err != nil {
		t.Fatalf("TryKeychainUnlock: %v", err)
	}
	if v2.Status() != domain.VaultStatusUnlocked {
		t.Fatalf("after TryKeychainUnlock: want Unlocked, got %v", v2.Status())
	}
	if dsn, err := v2.GetProfileDSN("p"); err != nil || dsn != "dsn" {
		t.Fatalf("GetProfileDSN after keychain unlock: dsn=%q err=%v", dsn, err)
	}

	if err := v2.DisableKeychain(ctx); err != nil {
		t.Fatalf("DisableKeychain: %v", err)
	}
	if v2.IsKeychainEnabled() {
		t.Fatalf("IsKeychainEnabled after DisableKeychain: want false")
	}
}

func TestVaultKeychainUnavailable(t *testing.T) {
	store := &memStore{}
	kc := &memKeychain{available: false}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	v, err := core.NewVault(ctx, store, kc, logger)
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	if err := v.Initialize(ctx, "pw"); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	if err := v.EnableKeychain(ctx); !errors.Is(err, errs.ErrKeychainUnavailable) {
		t.Fatalf("EnableKeychain when unavailable: want ErrKeychainUnavailable, got %v", err)
	}
	if v.IsKeychainEnabled() {
		t.Fatalf("IsKeychainEnabled when unavailable: want false")
	}
	// TryKeychainUnlock is idempotent when already Unlocked — Lock first so the
	// keychain path actually runs.
	v.Lock()
	if err := v.TryKeychainUnlock(ctx); !errors.Is(err, errs.ErrKeychainUnavailable) {
		t.Fatalf("TryKeychainUnlock when unavailable: want ErrKeychainUnavailable, got %v", err)
	}
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
