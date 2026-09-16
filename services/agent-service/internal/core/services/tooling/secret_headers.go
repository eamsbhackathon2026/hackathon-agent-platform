package tooling

import (
	"encoding/json"
	"net/http"
	"sort"

	"github.com/google/uuid"
)

func (s *Service) encryptHeaders(kind string, id uuid.UUID, headers map[string]string) ([]byte, []string, error) {
	if len(headers) == 0 {
		return nil, []string{}, nil
	}
	canonical := make(map[string]string, len(headers))
	names := make([]string, 0, len(headers))
	for name, value := range headers {
		name = canonicalHeaderName(name)
		canonical[name] = value
		names = append(names, name)
	}
	sort.Strings(names)
	plain, err := json.Marshal(canonical)
	if err != nil {
		return nil, nil, err
	}
	ciphertext, err := s.Cipher.Encrypt(plain, []byte(kind+":"+id.String()))
	return ciphertext, names, err
}

func (s *Service) decryptHeaders(kind string, id uuid.UUID, ciphertext []byte) (map[string]string, error) {
	if len(ciphertext) == 0 {
		return map[string]string{}, nil
	}
	plain, err := s.Cipher.Decrypt(ciphertext, []byte(kind+":"+id.String()))
	if err != nil {
		return nil, err
	}
	result := map[string]string{}
	if err = json.Unmarshal(plain, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func canonicalHeaderName(name string) string {
	return http.CanonicalHeaderKey(name)
}
