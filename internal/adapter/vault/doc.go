// Package vault is the secondary adapter for the master-password vault. It
// implements core.EncryptedStore by reading and writing a JSON-encoded
// VaultBlob at <UserConfigDir>/electrorangerd/vault.json, and core.Keychain
// by delegating to the OS keychain (macOS Keychain / Windows Credential
// Manager / Linux Secret Service) via build-tagged files. Plaintext DSNs
// never appear on disk — the blob is opaque AES-GCM ciphertext.
package vault
