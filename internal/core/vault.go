package core

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"golang.org/x/crypto/argon2"

	"github.com/rpatton4/electrorangerd/internal/domain"
	"github.com/rpatton4/electrorangerd/internal/errs"
	"github.com/rpatton4/electrorangerd/internal/port"
)

// EncryptedStore reads and writes the on-disk encrypted vault blob.
// Implementations return errs.ErrNotFound from Load when no vault file
// exists yet. Implemented by adapter/vault.
type EncryptedStore interface {
	Load(ctx context.Context) (domain.VaultBlob, error)
	Save(ctx context.Context, blob domain.VaultBlob) error
	Delete(ctx context.Context) error
}

// Keychain stores and retrieves the data-encryption-key from the OS keychain.
// Available reports whether the underlying OS keychain is reachable in the
// current process. Implemented by adapter/vault with per-OS build-tagged
// files (keychain_darwin.go / keychain_windows.go / keychain_linux.go).
type Keychain interface {
	Available() bool
	Set(ctx context.Context, dek []byte) error
	Get(ctx context.Context) ([]byte, error)
	Delete(ctx context.Context) error
}

const (
	vaultBlobVersion = 1
	dekLength        = 32 // AES-256 key
	saltLength       = 16
	nonceLength      = 12 // AES-GCM standard nonce length

	// Argon2id parameters per OWASP 2024 recommendation.
	argonTime    uint32 = 1
	argonMemory  uint32 = 64 * 1024 // 64 MB
	argonThreads uint8  = 4
)

type vaultService struct {
	store    EncryptedStore
	keychain Keychain
	log      *slog.Logger

	mu     sync.Mutex
	status domain.VaultStatus
	dek    []byte           // length dekLength when Unlocked; nil otherwise
	blob   domain.VaultBlob // last-known on-disk state; written through on every mutation
}

// NewVault constructs a Vault. It probes the EncryptedStore for an existing
// vault blob — if found, the vault starts in the Locked state; if not found,
// it starts in the Uninitialized state. Any I/O error from the probe is
// returned as a wrapped error so the composition root can decide whether to
// continue or exit.
func NewVault(ctx context.Context, store EncryptedStore, keychain Keychain, log *slog.Logger) (port.Vault, error) {
	v := &vaultService{
		store:    store,
		keychain: keychain,
		log:      log,
	}
	blob, err := store.Load(ctx)
	if errors.Is(err, errs.ErrNotFound) {
		v.status = domain.VaultStatusUninitialized
		return v, nil
	}
	if err != nil {
		return nil, fmt.Errorf("vault probe: %w", err)
	}
	v.blob = blob
	v.status = domain.VaultStatusLocked
	return v, nil
}

// Status returns the current vault state. Safe to call concurrently with
// other operations.
func (v *vaultService) Status() domain.VaultStatus {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.status
}

// Initialize creates a fresh vault: generates a random Argon2id salt and a
// random DEK, derives the KEK from password, wraps the DEK with the KEK, and
// persists the resulting blob. After a successful Initialize the vault is
// Unlocked. Returns errs.ErrVaultAlreadyInitialized if a vault blob already
// exists.
func (v *vaultService) Initialize(ctx context.Context, password string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.status != domain.VaultStatusUninitialized {
		return fmt.Errorf("vault initialize: %w", errs.ErrVaultAlreadyInitialized)
	}

	salt, err := randomBytes(saltLength)
	if err != nil {
		return fmt.Errorf("vault initialize: salt: %w", err)
	}
	dek, err := randomBytes(dekLength)
	if err != nil {
		return fmt.Errorf("vault initialize: dek: %w", err)
	}
	kdf := domain.KDFParams{Salt: salt, Time: argonTime, Memory: argonMemory, Threads: argonThreads}
	kek := deriveKEK(password, kdf)
	wrapped, err := encrypt(dek, kek)
	if err != nil {
		return fmt.Errorf("vault initialize: wrap dek: %w", err)
	}

	v.blob = domain.VaultBlob{
		Version:    vaultBlobVersion,
		KDFParams:  kdf,
		WrappedDEK: wrapped,
		Profiles:   nil,
	}
	if err := v.store.Save(ctx, v.blob); err != nil {
		return fmt.Errorf("vault initialize: save: %w", err)
	}
	v.dek = dek
	v.status = domain.VaultStatusUnlocked
	return nil
}

// Unlock derives the KEK from password using the stored Argon2id parameters,
// attempts to unwrap the DEK, and transitions to Unlocked on success. Returns
// errs.ErrInvalidPassword if AES-GCM authentication fails — that is the
// only signal of a wrong password. Returns errs.ErrVaultUninitialized if
// the vault was never initialized.
func (v *vaultService) Unlock(_ context.Context, password string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.status == domain.VaultStatusUninitialized {
		return fmt.Errorf("vault unlock: %w", errs.ErrVaultUninitialized)
	}
	if v.status == domain.VaultStatusUnlocked {
		return nil
	}
	kek := deriveKEK(password, v.blob.KDFParams)
	dek, err := decrypt(v.blob.WrappedDEK, kek)
	if err != nil {
		return fmt.Errorf("vault unlock: %w", errs.ErrInvalidPassword)
	}
	v.dek = dek
	v.status = domain.VaultStatusUnlocked
	return nil
}

// Lock zeroes the in-memory DEK and returns the vault to the Locked state.
// The on-disk blob is untouched. No-op if the vault was not Unlocked.
func (v *vaultService) Lock() {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.status != domain.VaultStatusUnlocked {
		return
	}
	zero(v.dek)
	v.dek = nil
	v.status = domain.VaultStatusLocked
}

// ChangePassword verifies the old password by attempting an unwrap, derives a
// new KEK from newPassword using a freshly generated salt, re-wraps the
// existing DEK with the new KEK, and persists the updated blob. The DEK and
// every encrypted profile DSN are unchanged — only WrappedDEK and KDFParams
// change. The vault must be Unlocked for this to succeed.
func (v *vaultService) ChangePassword(ctx context.Context, oldPassword, newPassword string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.status != domain.VaultStatusUnlocked {
		return fmt.Errorf("vault change password: %w", errs.ErrVaultLocked)
	}

	oldKEK := deriveKEK(oldPassword, v.blob.KDFParams)
	if _, err := decrypt(v.blob.WrappedDEK, oldKEK); err != nil {
		return fmt.Errorf("vault change password: %w", errs.ErrInvalidPassword)
	}

	newSalt, err := randomBytes(saltLength)
	if err != nil {
		return fmt.Errorf("vault change password: salt: %w", err)
	}
	newKDF := domain.KDFParams{Salt: newSalt, Time: argonTime, Memory: argonMemory, Threads: argonThreads}
	newKEK := deriveKEK(newPassword, newKDF)
	newWrapped, err := encrypt(v.dek, newKEK)
	if err != nil {
		return fmt.Errorf("vault change password: wrap: %w", err)
	}

	updated := v.blob
	updated.KDFParams = newKDF
	updated.WrappedDEK = newWrapped
	if err := v.store.Save(ctx, updated); err != nil {
		return fmt.Errorf("vault change password: save: %w", err)
	}
	v.blob = updated
	return nil
}

// GetProfileDSN returns the plaintext DSN for the named profile. The vault
// must be Unlocked. Returns errs.ErrNotFound when no such profile exists.
func (v *vaultService) GetProfileDSN(name string) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.status != domain.VaultStatusUnlocked {
		return "", fmt.Errorf("vault get profile: %w", errs.ErrVaultLocked)
	}
	for _, p := range v.blob.Profiles {
		if p.Name == name {
			plaintext, err := decrypt(p.EncryptedDSN, v.dek)
			if err != nil {
				return "", fmt.Errorf("vault get profile %q: decrypt: %w", name, err)
			}
			return string(plaintext), nil
		}
	}
	return "", fmt.Errorf("vault get profile %q: %w", name, errs.ErrNotFound)
}

// SaveProfileDSN encrypts dsn with the DEK and upserts the named profile,
// then persists the updated blob.
func (v *vaultService) SaveProfileDSN(ctx context.Context, name, dsn string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.status != domain.VaultStatusUnlocked {
		return fmt.Errorf("vault save profile: %w", errs.ErrVaultLocked)
	}
	encrypted, err := encrypt([]byte(dsn), v.dek)
	if err != nil {
		return fmt.Errorf("vault save profile %q: encrypt: %w", name, err)
	}
	updated := v.blob
	updated.Profiles = upsertProfile(v.blob.Profiles, domain.ConnectionProfile{Name: name, EncryptedDSN: encrypted})
	if err := v.store.Save(ctx, updated); err != nil {
		return fmt.Errorf("vault save profile %q: save: %w", name, err)
	}
	v.blob = updated
	return nil
}

// DeleteProfile removes the named profile and persists the updated blob.
// Returns errs.ErrNotFound if no such profile exists.
func (v *vaultService) DeleteProfile(ctx context.Context, name string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.status != domain.VaultStatusUnlocked {
		return fmt.Errorf("vault delete profile: %w", errs.ErrVaultLocked)
	}
	idx := -1
	for i, p := range v.blob.Profiles {
		if p.Name == name {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("vault delete profile %q: %w", name, errs.ErrNotFound)
	}
	updated := v.blob
	updated.Profiles = append(append([]domain.ConnectionProfile(nil), v.blob.Profiles[:idx]...), v.blob.Profiles[idx+1:]...)
	if err := v.store.Save(ctx, updated); err != nil {
		return fmt.Errorf("vault delete profile %q: save: %w", name, err)
	}
	v.blob = updated
	return nil
}

// ListProfiles returns the names of every stored profile. Safe to call when
// the vault is Locked — names are not secret, only DSNs are.
func (v *vaultService) ListProfiles() ([]string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.status == domain.VaultStatusUninitialized {
		return nil, fmt.Errorf("vault list profiles: %w", errs.ErrVaultUninitialized)
	}
	names := make([]string, len(v.blob.Profiles))
	for i, p := range v.blob.Profiles {
		names[i] = p.Name
	}
	return names, nil
}

// EnableKeychain stores the current DEK in the OS keychain so the next
// launch can auto-unlock via TryKeychainUnlock without prompting for a
// password. The vault must be Unlocked.
func (v *vaultService) EnableKeychain(ctx context.Context) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.status != domain.VaultStatusUnlocked {
		return fmt.Errorf("vault enable keychain: %w", errs.ErrVaultLocked)
	}
	if !v.keychain.Available() {
		return fmt.Errorf("vault enable keychain: %w", errs.ErrKeychainUnavailable)
	}
	if err := v.keychain.Set(ctx, v.dek); err != nil {
		return fmt.Errorf("vault enable keychain: %w", err)
	}
	return nil
}

// DisableKeychain removes the DEK from the OS keychain if present. Safe to
// call regardless of vault status — the operation is about the keychain
// entry, not the in-memory state.
func (v *vaultService) DisableKeychain(ctx context.Context) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if !v.keychain.Available() {
		return fmt.Errorf("vault disable keychain: %w", errs.ErrKeychainUnavailable)
	}
	if err := v.keychain.Delete(ctx); err != nil && !errors.Is(err, errs.ErrNotFound) {
		return fmt.Errorf("vault disable keychain: %w", err)
	}
	return nil
}

// IsKeychainEnabled reports whether a DEK is currently stored in the OS
// keychain. False also covers the case where the keychain is unavailable.
func (v *vaultService) IsKeychainEnabled() bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	if !v.keychain.Available() {
		return false
	}
	_, err := v.keychain.Get(context.Background())
	return err == nil
}

// TryKeychainUnlock reads the DEK from the OS keychain and transitions to
// Unlocked. Returns errs.ErrKeychainUnavailable if the keychain is not
// reachable, or errs.ErrNotFound if no DEK is stored. Returns
// errs.ErrVaultUninitialized if the vault was never initialized.
func (v *vaultService) TryKeychainUnlock(ctx context.Context) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.status == domain.VaultStatusUninitialized {
		return fmt.Errorf("vault keychain unlock: %w", errs.ErrVaultUninitialized)
	}
	if v.status == domain.VaultStatusUnlocked {
		return nil
	}
	if !v.keychain.Available() {
		return fmt.Errorf("vault keychain unlock: %w", errs.ErrKeychainUnavailable)
	}
	dek, err := v.keychain.Get(ctx)
	if err != nil {
		return fmt.Errorf("vault keychain unlock: %w", err)
	}
	if len(dek) != dekLength {
		return fmt.Errorf("vault keychain unlock: dek length %d, want %d: %w", len(dek), dekLength, errs.ErrInvalidPassword)
	}
	v.dek = dek
	v.status = domain.VaultStatusUnlocked
	return nil
}

// deriveKEK runs Argon2id over password and the KDF parameters to produce a
// 32-byte AES-256 key-encryption-key.
func deriveKEK(password string, kdf domain.KDFParams) []byte {
	return argon2.IDKey([]byte(password), kdf.Salt, kdf.Time, kdf.Memory, kdf.Threads, dekLength)
}

// encrypt produces nonce || ciphertext || auth-tag using AES-GCM with key.
func encrypt(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	nonce, err := randomBytes(nonceLength)
	if err != nil {
		return nil, fmt.Errorf("nonce: %w", err)
	}
	sealed := gcm.Seal(nil, nonce, plaintext, nil)
	out := make([]byte, 0, len(nonce)+len(sealed))
	out = append(out, nonce...)
	out = append(out, sealed...)
	return out, nil
}

// decrypt parses nonce || ciphertext || auth-tag and decrypts with key.
// Returns an error (which Unlock translates to errs.ErrInvalidPassword) when
// the auth tag does not validate.
func decrypt(blob, key []byte) ([]byte, error) {
	if len(blob) < nonceLength {
		return nil, fmt.Errorf("ciphertext too short")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	nonce := blob[:nonceLength]
	ct := blob[nonceLength:]
	plaintext, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("auth: %w", err)
	}
	return plaintext, nil
}

func randomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

func upsertProfile(profiles []domain.ConnectionProfile, p domain.ConnectionProfile) []domain.ConnectionProfile {
	for i, existing := range profiles {
		if existing.Name == p.Name {
			out := append([]domain.ConnectionProfile(nil), profiles...)
			out[i] = p
			return out
		}
	}
	return append(append([]domain.ConnectionProfile(nil), profiles...), p)
}
