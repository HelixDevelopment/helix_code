package llm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewQwenOAuth2Client(t *testing.T) {
	client := NewQwenOAuth2Client()
	require.NotNil(t, client)
	assert.NotNil(t, client.httpClient)
	assert.Equal(t, "https://chat.qwen.ai", client.baseURL)
	assert.NotEmpty(t, client.clientID)
}

func TestQwenOAuth2Client_GenerateCodeVerifier(t *testing.T) {
	t.Run("GeneratesRandomVerifier", func(t *testing.T) {
		client := NewQwenOAuth2Client()

		verifier1, err := client.GenerateCodeVerifier()
		require.NoError(t, err)
		assert.NotEmpty(t, verifier1)
		assert.GreaterOrEqual(t, len(verifier1), 32) // At least 32 chars

		// Generate another one to verify randomness
		verifier2, err := client.GenerateCodeVerifier()
		require.NoError(t, err)
		assert.NotEqual(t, verifier1, verifier2)
	})
}

func TestQwenOAuth2Client_GenerateCodeChallenge(t *testing.T) {
	t.Run("GeneratesChallenge", func(t *testing.T) {
		client := NewQwenOAuth2Client()

		verifier := "test-verifier-string"
		challenge := client.GenerateCodeChallenge(verifier)
		assert.NotEmpty(t, challenge)
	})

	t.Run("SameVerifierSameChallenge", func(t *testing.T) {
		client := NewQwenOAuth2Client()

		verifier := "consistent-verifier"
		challenge1 := client.GenerateCodeChallenge(verifier)
		challenge2 := client.GenerateCodeChallenge(verifier)

		assert.Equal(t, challenge1, challenge2)
	})

	t.Run("DifferentVerifierDifferentChallenge", func(t *testing.T) {
		client := NewQwenOAuth2Client()

		challenge1 := client.GenerateCodeChallenge("verifier1")
		challenge2 := client.GenerateCodeChallenge("verifier2")

		assert.NotEqual(t, challenge1, challenge2)
	})
}

func TestQwenOAuth2Client_RequestDeviceAuthorization(t *testing.T) {
	t.Run("SuccessfulAuthorization", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/oauth2/device/code" {
				json.NewEncoder(w).Encode(DeviceAuthorizationData{
					DeviceCode:              "test-device-code",
					UserCode:                "ABCD-EFGH",
					VerificationURI:         "https://example.com/verify",
					VerificationURIComplete: "https://example.com/verify?code=ABCD-EFGH",
					ExpiresIn:               300,
				})
			}
		}))
		defer server.Close()

		client := NewQwenOAuth2Client()
		client.baseURL = server.URL

		data, err := client.RequestDeviceAuthorization("openid", "test-challenge")
		require.NoError(t, err)
		assert.Equal(t, "test-device-code", data.DeviceCode)
		assert.Equal(t, "ABCD-EFGH", data.UserCode)
	})

	t.Run("ServerError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		client := NewQwenOAuth2Client()
		client.baseURL = server.URL

		data, err := client.RequestDeviceAuthorization("openid", "test-challenge")
		assert.Error(t, err)
		assert.Nil(t, data)
	})
}

func TestQwenOAuth2Client_PollDeviceToken(t *testing.T) {
	t.Run("SuccessfulTokenRetrieval", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Handle both the token endpoint and potential other calls
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(DeviceTokenData{
				AccessToken:  "test-access-token",
				RefreshToken: "test-refresh-token",
				TokenType:    "Bearer",
				ExpiresIn:    3600,
			})
		}))
		defer server.Close()

		client := NewQwenOAuth2Client()
		client.baseURL = server.URL

		data, err := client.PollDeviceToken("test-device-code", "test-verifier")
		require.NoError(t, err)
		assert.Equal(t, "test-access-token", data.AccessToken)
		assert.Equal(t, "test-refresh-token", data.RefreshToken)
		assert.Equal(t, "Bearer", data.TokenType)
	})

	t.Run("AuthorizationPending", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ErrorData{
				Error:            "authorization_pending",
				ErrorDescription: "The authorization request is still pending",
			})
		}))
		defer server.Close()

		client := NewQwenOAuth2Client()
		client.baseURL = server.URL

		data, err := client.PollDeviceToken("test-device-code", "test-verifier")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "authorization_pending")
		assert.Nil(t, data)
	})
}

func TestQwenOAuth2Client_RefreshAccessToken(t *testing.T) {
	t.Run("SuccessfulRefresh", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/oauth2/token" {
				json.NewEncoder(w).Encode(DeviceTokenData{
					AccessToken:  "new-access-token",
					RefreshToken: "new-refresh-token",
					TokenType:    "Bearer",
					ExpiresIn:    3600,
				})
			}
		}))
		defer server.Close()

		client := NewQwenOAuth2Client()
		client.baseURL = server.URL

		data, err := client.RefreshAccessToken("old-refresh-token")
		require.NoError(t, err)
		assert.Equal(t, "new-access-token", data.AccessToken)
	})

	t.Run("InvalidRefreshToken", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ErrorData{
				Error:            "invalid_grant",
				ErrorDescription: "The refresh token is invalid",
			})
		}))
		defer server.Close()

		client := NewQwenOAuth2Client()
		client.baseURL = server.URL

		data, err := client.RefreshAccessToken("invalid-token")
		assert.Error(t, err)
		assert.Nil(t, data)
	})
}

func TestQwenOAuth2Client_GetCredentialsPath(t *testing.T) {
	t.Run("ReturnsPath", func(t *testing.T) {
		client := NewQwenOAuth2Client()
		path := client.GetCredentialsPath()
		assert.NotEmpty(t, path)
		// Path contains .qwen directory and some form of credentials file
		assert.Contains(t, path, ".qwen")
	})
}

func TestQwenOAuth2Client_IsTokenValid(t *testing.T) {
	t.Run("NilCredentials", func(t *testing.T) {
		client := NewQwenOAuth2Client()
		assert.False(t, client.IsTokenValid(nil))
	})

	t.Run("EmptyAccessToken", func(t *testing.T) {
		client := NewQwenOAuth2Client()
		creds := &QwenCredentials{
			AccessToken: "",
		}
		assert.False(t, client.IsTokenValid(creds))
	})

	t.Run("ExpiredToken", func(t *testing.T) {
		client := NewQwenOAuth2Client()
		creds := &QwenCredentials{
			AccessToken: "test-token",
			ExpiryDate:  1, // Very old timestamp
		}
		assert.False(t, client.IsTokenValid(creds))
	})
}

func TestQwenCredentials_Struct(t *testing.T) {
	creds := QwenCredentials{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		IDToken:      "id-token",
		ExpiryDate:   1704067200,
		TokenType:    "Bearer",
		ResourceURL:  "https://resource.example.com",
	}

	assert.Equal(t, "access-token", creds.AccessToken)
	assert.Equal(t, "refresh-token", creds.RefreshToken)
	assert.Equal(t, "id-token", creds.IDToken)
	assert.Equal(t, int64(1704067200), creds.ExpiryDate)
	assert.Equal(t, "Bearer", creds.TokenType)
	assert.Equal(t, "https://resource.example.com", creds.ResourceURL)
}

func TestDeviceAuthorizationData_Struct(t *testing.T) {
	data := DeviceAuthorizationData{
		DeviceCode:              "device-code",
		UserCode:                "USER-CODE",
		VerificationURI:         "https://verify.example.com",
		VerificationURIComplete: "https://verify.example.com?code=USER-CODE",
		ExpiresIn:               300,
	}

	assert.Equal(t, "device-code", data.DeviceCode)
	assert.Equal(t, "USER-CODE", data.UserCode)
	assert.Equal(t, "https://verify.example.com", data.VerificationURI)
	assert.Equal(t, 300, data.ExpiresIn)
}

func TestDeviceTokenData_Struct(t *testing.T) {
	data := DeviceTokenData{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		TokenType:    "Bearer",
		ExpiresIn:    3600,
		Scope:        "openid profile",
		Endpoint:     "https://api.example.com",
		ResourceURL:  "https://resource.example.com",
	}

	assert.Equal(t, "access-token", data.AccessToken)
	assert.Equal(t, "refresh-token", data.RefreshToken)
	assert.Equal(t, "Bearer", data.TokenType)
	assert.Equal(t, 3600, data.ExpiresIn)
	assert.Equal(t, "openid profile", data.Scope)
}

func TestErrorData_Struct(t *testing.T) {
	data := ErrorData{
		Error:            "invalid_request",
		ErrorDescription: "The request was invalid",
	}

	assert.Equal(t, "invalid_request", data.Error)
	assert.Equal(t, "The request was invalid", data.ErrorDescription)
}
