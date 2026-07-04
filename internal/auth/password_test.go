package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Password hashing is security-critical and pure (no dependencies), so it is a
// cheap, fast, high-value unit test — the reference pattern for Go unit tests
// in this repo. See TESTING.md.

func TestEncodePasswordAndMatches(t *testing.T) {
	tests := []struct {
		name     string
		password string
	}{
		{name: "simple", password: "hunter2"},
		{name: "with spaces", password: "correct horse battery staple"},
		{name: "unicode", password: "pÄsswörd🔒"},
		{name: "long", password: "aVeryLongPasswordThatGoesOnAndOnAndOn1234567890"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hash, err := EncodePassword(tc.password)
			require.NoError(t, err)
			require.NotEmpty(t, hash)
			assert.NotEqual(t, tc.password, hash, "hash must not equal the plaintext")

			// Correct password verifies.
			assert.NoError(t, Matches(hash, tc.password), "correct password should match")

			// Wrong password is rejected.
			assert.Error(t, Matches(hash, tc.password+"x"), "wrong password should not match")
		})
	}
}

func TestEncodePasswordIsSalted(t *testing.T) {
	// bcrypt salts each hash, so the same password must produce different hashes,
	// while both still verify. This guards against a regression to unsalted hashing.
	h1, err := EncodePassword("samepassword")
	require.NoError(t, err)
	h2, err := EncodePassword("samepassword")
	require.NoError(t, err)

	assert.NotEqual(t, h1, h2, "two hashes of the same password must differ (salted)")
	assert.NoError(t, Matches(h1, "samepassword"))
	assert.NoError(t, Matches(h2, "samepassword"))
}

func TestGenerateRandomPassword(t *testing.T) {
	for _, length := range []int{1, 8, 16, 64} {
		got := GenerateRandomPassword(length)
		assert.Len(t, got, length, "generated password should have the requested length")
	}

	// Two generated passwords of a reasonable length should not collide.
	assert.NotEqual(t, GenerateRandomPassword(32), GenerateRandomPassword(32),
		"random passwords should not collide")
}
