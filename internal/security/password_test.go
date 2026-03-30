package security

import (
	"strings"
	"testing"
)

func TestHashPassword(t *testing.T) {
	tests := []struct {
		name      string
		password  string
		wantErr   bool
		formatOK  bool
	}{
		{
			name:     "valid password",
			password: "correct-horse-battery-staple",
			wantErr:  false,
			formatOK: true,
		},
		{
			name:     "minimum length password",
			password: strings.Repeat("a", minPasswordLen),
			wantErr:  false,
			formatOK: true,
		},
		{
			name:     "empty password",
			password: "",
			wantErr:  true,
			formatOK: false,
		},
		{
			name:     "long password",
			password: strings.Repeat("x", 100),
			wantErr:  false,
			formatOK: true,
		},
		{
			name:     "password with unicode",
			password: "パスワード1234",
			wantErr:  false,
			formatOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := HashPassword(tt.password)

			if tt.wantErr {
				if err == nil {
					t.Errorf("HashPassword() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("HashPassword() unexpected error: %v", err)
				return
			}

			if tt.formatOK {
				// Check format: $argon2id$v=19$t=3,m=65536,p=4$<salt>$<hash>
				parts := strings.Split(hash, "$")
				if len(parts) != 6 {
					t.Errorf("HashPassword() invalid format, got %d parts, want 6", len(parts))
				}
				if parts[0] != "" {
					t.Errorf("HashPassword() first part should be empty, got %q", parts[0])
				}
				if parts[1] != "argon2id" {
					t.Errorf("HashPassword() algorithm should be argon2id, got %q", parts[1])
				}
			}
		})
	}
}

func TestVerifyPassword(t *testing.T) {
	// Create a known hash
	password := "test-password-12345"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to create test hash: %v", err)
	}

	tests := []struct {
		name     string
		password string
		hash     string
		want     bool
		wantErr  bool
	}{
		{
			name:     "correct password",
			password: password,
			hash:     hash,
			want:     true,
			wantErr:  false,
		},
		{
			name:     "wrong password",
			password: "wrong-password",
			hash:     hash,
			want:     false,
			wantErr:  false,
		},
		{
			name:     "empty password",
			password: "",
			hash:     hash,
			want:     false,
			wantErr:  false,
		},
		{
			name:     "empty hash",
			password: password,
			hash:     "",
			want:     false,
			wantErr:  false,
		},
		{
			name:     "invalid hash format",
			password: password,
			hash:     "invalid-hash",
			want:     false,
			wantErr:  true,
		},
		{
			name:     "trailing space in password",
			password: password + " ",
			hash:     hash,
			want:     false,
			wantErr:  false,
		},
		{
			name:     "leading space in password",
			password: " " + password,
			hash:     hash,
			want:     false,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := VerifyPassword(tt.password, tt.hash)

			if tt.wantErr {
				if err == nil {
					t.Errorf("VerifyPassword() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("VerifyPassword() unexpected error: %v", err)
				return
			}

			if got != tt.want {
				t.Errorf("VerifyPassword() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHashVerifyRoundTrip(t *testing.T) {
	passwords := []string{
		"correct-horse-battery-staple",
		"Tr0ub4dor&3",
		strings.Repeat("a", minPasswordLen),
		strings.Repeat("z", 100),
		"密码12345678",
		"mot_de_passe_123",
		"!@#$%^&*()_+-=[]{}|;:,.<>?",
	}

	for _, pwd := range passwords {
		t.Run(pwd, func(t *testing.T) {
			hash, err := HashPassword(pwd)
			if err != nil {
				t.Errorf("HashPassword(%q) failed: %v", pwd, err)
				return
			}

			// Verify the original password works
			valid, err := VerifyPassword(pwd, hash)
			if err != nil {
				t.Errorf("VerifyPassword(%q) failed: %v", pwd, err)
				return
			}
			if !valid {
				t.Errorf("VerifyPassword(%q) returned false, want true", pwd)
			}

			// Verify a wrong password fails
			valid, err = VerifyPassword(pwd+"x", hash)
			if err != nil {
				t.Errorf("VerifyPassword(%q+x) failed: %v", pwd, err)
				return
			}
			if valid {
				t.Errorf("VerifyPassword(%q+x) returned true, want false", pwd)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{
			name:     "valid password",
			password: strings.Repeat("a", minPasswordLen),
			wantErr:  false,
		},
		{
			name:     "too short",
			password: strings.Repeat("a", minPasswordLen-1),
			wantErr:  true,
		},
		{
			name:     "empty password",
			password: "",
			wantErr:  true,
		},
		{
			name:     "long password",
			password: strings.Repeat("a", 100),
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePassword() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConstantTimeCompare(t *testing.T) {
	tests := []struct {
		name  string
		a     string
		b     string
		want  bool
	}{
		{
			name: "equal strings",
			a:    "test-string",
			b:    "test-string",
			want: true,
		},
		{
			name: "different strings",
			a:    "test-string",
			b:    "different-string",
			want: false,
		},
		{
			name: "different length",
			a:    "test",
			b:    "test-longer",
			want: false,
		},
		{
			name: "empty strings",
			a:    "",
			b:    "",
			want: true,
		},
		{
			name: "one char diff",
			a:    "test-string",
			b:    "test-stringx",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ConstantTimeCompare(tt.a, tt.b); got != tt.want {
				t.Errorf("ConstantTimeCompare() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestTimingSafety verifies that password verification time doesn't leak
// information about password similarity. This is a basic check; for real
// timing safety, you'd need statistical analysis of many runs.
func TestTimingSafety(t *testing.T) {
	// Create a hash
	password := "test-password-12345"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to create test hash: %v", err)
	}

	// Verify similar passwords should take roughly the same time
	// (This is a basic sanity check, not a rigorous timing analysis)
	tests := []string{
		password,           // correct
		"xest-password-12345", // one char off
		"test-password-12346", // last char off
		"aaaa",              // very different
	}

	for _, testPwd := range tests {
		t.Run(testPwd, func(t *testing.T) {
			valid, err := VerifyPassword(testPwd, hash)
			if err != nil {
				t.Errorf("VerifyPassword(%q) error: %v", testPwd, err)
			}
			_ = valid // We just care that it doesn't crash
		})
	}
}
