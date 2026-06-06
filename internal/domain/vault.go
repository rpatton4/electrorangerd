package domain

// VaultStatus is the current state of the master vault.
type VaultStatus int

const (
	// VaultStatusUninitialized means no vault file exists yet — the user has
	// not set a master password.
	VaultStatusUninitialized VaultStatus = iota
	// VaultStatusLocked means a vault file exists but the master password has
	// not been supplied this session, so secrets cannot be decrypted.
	VaultStatusLocked
	// VaultStatusUnlocked means the data encryption key is held in memory and
	// secrets can be encrypted and decrypted.
	VaultStatusUnlocked
)

// KDFParams captures the Argon2id parameters used to derive the key-
// encryption-key from the master password. Salt is generated once at vault
// initialization and persisted alongside Time / Memory / Threads so the
// derivation is reproducible across launches.
type KDFParams struct {
	Salt    []byte `json:"salt"`
	Time    uint32 `json:"time"`
	Memory  uint32 `json:"memory"`
	Threads uint8  `json:"threads"`
}

// VaultBlob is the on-disk representation of the encrypted vault. It holds the
// KDF parameters, the data-encryption-key wrapped by the key-encryption-key
// derived from the master password, and the list of connection profiles whose
// DSNs are encrypted with the data-encryption-key.
//
// Password change re-encrypts only WrappedDEK (the small key blob), not every
// profile — the DEK / KEK split keeps the password-change operation cheap.
type VaultBlob struct {
	Version    int                 `json:"version"`
	KDFParams  KDFParams           `json:"kdf"`
	WrappedDEK []byte              `json:"wrapped_dek"`
	Profiles   []ConnectionProfile `json:"profiles"`
}

// ConnectionProfile holds a named database connection. The DSN is stored as
// an AES-GCM ciphertext (nonce || ciphertext || auth-tag) encrypted with the
// data-encryption-key held by the unlocked vault. Plaintext DSNs never appear
// on disk.
type ConnectionProfile struct {
	Name         string `json:"name"`
	EncryptedDSN []byte `json:"encrypted_dsn"`
}
