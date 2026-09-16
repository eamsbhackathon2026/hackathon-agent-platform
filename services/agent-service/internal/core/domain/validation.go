package domain

import (
	"net/mail"
	"strings"
	"unicode/utf8"
)

// NormalizeEmail accepts a single mailbox without display names.
func NormalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || address.Name != "" || len(email) > 254 {
		return "", Invalid("email", "Địa chỉ email không hợp lệ.")
	}
	return email, nil
}

// ValidateName checks human-readable names using Unicode character counts.
func ValidateName(field, value string) error {
	n := utf8.RuneCountInString(strings.TrimSpace(value))
	if !utf8.ValidString(value) || n < 1 || n > 200 {
		return Invalid(field, "Tên phải có từ 1 đến 200 ký tự.")
	}
	return nil
}

// ValidatePassword bounds Unicode password length before expensive hashing.
func ValidatePassword(value string) error {
	n := utf8.RuneCountInString(value)
	if !utf8.ValidString(value) || n < 10 || n > 1024 {
		return Invalid("password", "Mật khẩu phải có từ 10 đến 1024 ký tự.")
	}
	return nil
}

// ValidRole reports whether a role belongs to the supported policy matrix.
func ValidRole(role Role) bool { return role == RoleOwner || role == RoleAdmin || role == RoleMember }
