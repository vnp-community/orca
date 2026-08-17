package usecase

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/stablyai/orca-go/services/credential-broker-service/internal/domain"
)

// callRecorder is a shared, ordered log of "which fake method fired" —
// used across fakeMetadataRepo/fakeAuditRepo/fakeSecretStore in this
// package's tests to assert cross-fake call ORDER (e.g. "Vault was read
// before the audit entry was appended, and the audit entry was appended
// before Execute returned"), not just that each call happened at all.
type callRecorder struct {
	mu    sync.Mutex
	calls []string
}

func (r *callRecorder) record(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, name)
}

func (r *callRecorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.calls))
	copy(out, r.calls)
	return out
}

// fakeMetadataRepo is an in-memory CredentialMetadataRepository.
type fakeMetadataRepo struct {
	recorder  *callRecorder
	rows      map[string]domain.CredentialMetadata
	createErr error
	getErr    error
	updateErr error
}

func newFakeMetadataRepo(r *callRecorder) *fakeMetadataRepo {
	return &fakeMetadataRepo{recorder: r, rows: make(map[string]domain.CredentialMetadata)}
}

func (f *fakeMetadataRepo) Create(ctx context.Context, m domain.CredentialMetadata) error {
	f.recorder.record("metadata.Create")
	if f.createErr != nil {
		return f.createErr
	}
	f.rows[m.ID] = m
	return nil
}

func (f *fakeMetadataRepo) Get(ctx context.Context, id string) (domain.CredentialMetadata, error) {
	f.recorder.record("metadata.Get")
	if f.getErr != nil {
		return domain.CredentialMetadata{}, f.getErr
	}
	m, ok := f.rows[id]
	if !ok {
		return domain.CredentialMetadata{}, domain.ErrCredentialNotFound
	}
	return m, nil
}

func (f *fakeMetadataRepo) UpdateStatus(ctx context.Context, id string, status domain.Status, now time.Time) error {
	f.recorder.record("metadata.UpdateStatus")
	if f.updateErr != nil {
		return f.updateErr
	}
	m, ok := f.rows[id]
	if !ok {
		return domain.ErrCredentialNotFound
	}
	m.Status = status
	m.UpdatedAt = now
	f.rows[id] = m
	return nil
}

// fakeAuditRepo is an in-memory AuditRepository.
type fakeAuditRepo struct {
	recorder  *callRecorder
	entries   []domain.AccessAuditEntry
	appendErr error
}

func newFakeAuditRepo(r *callRecorder) *fakeAuditRepo {
	return &fakeAuditRepo{recorder: r}
}

func (f *fakeAuditRepo) Append(ctx context.Context, e domain.AccessAuditEntry) error {
	f.recorder.record("audit.Append")
	if f.appendErr != nil {
		return f.appendErr
	}
	f.entries = append(f.entries, e)
	return nil
}

// fakeSecretStore is an in-memory SecretStore — never touches a real Vault.
type fakeSecretStore struct {
	recorder *callRecorder
	kv       map[string]map[string]any

	encryptErr error
	decryptErr error
	kvWriteErr error
	kvReadErr  error
	revokeErr  error

	revokeCalls []string // "mount/path" values RevokeSecret was called with
}

func newFakeSecretStore(r *callRecorder) *fakeSecretStore {
	return &fakeSecretStore{recorder: r, kv: make(map[string]map[string]any)}
}

func kvKey(mount, path string) string { return mount + "/" + path }

func (f *fakeSecretStore) TransitEncrypt(ctx context.Context, keyName string, plaintext []byte) (string, error) {
	f.recorder.record("store.TransitEncrypt")
	if f.encryptErr != nil {
		return "", f.encryptErr
	}
	return "vault:v1:" + keyName + ":" + string(plaintext), nil
}

func (f *fakeSecretStore) TransitDecrypt(ctx context.Context, keyName, ciphertext string) ([]byte, error) {
	f.recorder.record("store.TransitDecrypt")
	if f.decryptErr != nil {
		return nil, f.decryptErr
	}
	prefix := "vault:v1:" + keyName + ":"
	if len(ciphertext) < len(prefix) {
		return nil, errors.New("fake: malformed ciphertext")
	}
	return []byte(ciphertext[len(prefix):]), nil
}

func (f *fakeSecretStore) KVWrite(ctx context.Context, mount, path string, data map[string]any) error {
	f.recorder.record("store.KVWrite")
	if f.kvWriteErr != nil {
		return f.kvWriteErr
	}
	f.kv[kvKey(mount, path)] = data
	return nil
}

func (f *fakeSecretStore) KVRead(ctx context.Context, mount, path string) (map[string]any, error) {
	f.recorder.record("store.KVRead")
	if f.kvReadErr != nil {
		return nil, f.kvReadErr
	}
	data, ok := f.kv[kvKey(mount, path)]
	if !ok {
		return nil, errors.New("fake: not found")
	}
	return data, nil
}

func (f *fakeSecretStore) RevokeSecret(ctx context.Context, mount, path string) error {
	f.recorder.record("store.RevokeSecret")
	f.revokeCalls = append(f.revokeCalls, kvKey(mount, path))
	if f.revokeErr != nil {
		return f.revokeErr
	}
	delete(f.kv, kvKey(mount, path))
	return nil
}
