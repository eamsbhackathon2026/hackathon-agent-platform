package identity

import (
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"context"
	"encoding/base64"
	"errors"
	"github.com/google/uuid"
	"strings"
	"time"
)

// ListAPIKeys lists safe metadata using descending keyset pagination.
func (s *Service) ListAPIKeys(ctx context.Context, p domain.Principal, page inbound.PageRequest) (inbound.APIKeyPage, error) {
	var out inbound.APIKeyPage
	if err := domain.Authorize(p, domain.ActionAPIKeysRead, nil); err != nil {
		return out, err
	}
	options, err := pageOptions(page)
	if err != nil {
		return out, err
	}
	out.Items, err = s.APIKeys.ListAPIKeys(ctx, options)
	if err != nil {
		return out, err
	}
	if len(out.Items) == options.Limit {
		out.Items = out.Items[:options.Limit-1]
		last := out.Items[len(out.Items)-1]
		out.NextCursor = encodeCursor(domain.PageCursor{ID: last.ID, CreatedAt: last.CreatedAt})
	}
	for i := range out.Items {
		out.Items[i].KeyHash = nil
		out.Items[i].WebhookSecretCiphertext = nil
	}
	return out, nil
}

// CreateAPIKey returns new access and webhook secrets exactly once.
func (s *Service) CreateAPIKey(ctx context.Context, p domain.Principal, c inbound.APIKeyCreateCommand) (inbound.APIKeyCreated, error) {
	var out inbound.APIKeyCreated
	if err := domain.Authorize(p, domain.ActionAPIKeysWrite, nil); err != nil {
		return out, err
	}
	if err := domain.ValidateName("name", c.Name); err != nil {
		return out, err
	}
	scopes := make([]string, 0, 2)
	seen := make(map[string]bool)
	if len(c.Scopes) == 0 {
		return out, domain.Invalid("scopes", "Chọn ít nhất một quyền sử dụng.")
	}
	for _, v := range c.Scopes {
		if v != "runs:write" && v != "runs:read" {
			return out, domain.Invalid("scopes", "Quyền sử dụng không hợp lệ.")
		}
		if !seen[v] {
			scopes = append(scopes, v)
			seen[v] = true
		}
	}
	id, err := s.IDs.NewID()
	if err != nil {
		return out, err
	}
	prefix, err := s.Random.Base62(8)
	if err != nil {
		return out, err
	}
	secret, err := s.Random.Base62(43)
	if err != nil {
		return out, err
	}
	raw := "apk_" + prefix + "_" + secret
	webhookBytes, err := s.Random.Bytes(32)
	if err != nil {
		return out, err
	}
	webhook := "whsec_" + base64.StdEncoding.EncodeToString(webhookBytes)
	encrypted, err := s.Cipher.Encrypt([]byte(webhook), []byte("webhook:"+id.String()))
	if err != nil {
		return out, err
	}
	key := domain.APIKey{ID: id, Name: strings.TrimSpace(c.Name), Prefix: prefix, KeyHash: s.KeyHasher.Hash(raw), Scopes: scopes, WebhookSecretCiphertext: encrypted, CreatedBy: p.UserID, CreatedAt: s.Clock.Now()}
	if err = s.APIKeys.CreateAPIKey(ctx, key); err != nil {
		return out, err
	}
	key.KeyHash = nil
	key.WebhookSecretCiphertext = nil
	return inbound.APIKeyCreated{APIKey: key, Key: raw, WebhookSecret: webhook}, nil
}

// RevokeAPIKey disables an access key immediately.
func (s *Service) RevokeAPIKey(ctx context.Context, p domain.Principal, id uuid.UUID) error {
	if err := domain.Authorize(p, domain.ActionAPIKeysWrite, nil); err != nil {
		return err
	}
	return s.APIKeys.RevokeAPIKey(ctx, id, s.Clock.Now())
}
func validBase62(v string) bool {
	for _, c := range v {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') {
			return false
		}
	}
	return true
}

// FromAPIKey verifies a complete key and loads its current revocation state.
func (s *Service) FromAPIKey(ctx context.Context, raw string) (domain.Principal, error) {
	var out domain.Principal
	if len(raw) != 56 || !strings.HasPrefix(raw, "apk_") || raw[12] != '_' || !validBase62(raw[4:12]) || !validBase62(raw[13:]) {
		return out, domain.ErrUnauthenticated
	}
	key, err := s.APIKeys.FindAPIKeyByPrefix(ctx, raw[4:12])
	if errors.Is(err, domain.ErrNotFound) {
		return out, domain.ErrUnauthenticated
	}
	if err != nil {
		return out, err
	}
	if !s.KeyHasher.Verify(raw, key.KeyHash) || key.RevokedAt != nil {
		return out, domain.ErrUnauthenticated
	}
	now := s.Clock.Now()
	if key.LastUsedAt == nil || now.Sub(*key.LastUsedAt) >= time.Minute {
		if err = s.APIKeys.TouchAPIKey(ctx, key.ID, now); err != nil {
			return out, err
		}
	}
	return domain.Principal{Kind: domain.PrincipalAPIKey, APIKeyID: key.ID, Scopes: key.Scopes}, nil
}
