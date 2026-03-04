package auth

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestHashPassword(t *testing.T) {

	cases := []struct {
		password string
	}{
		{
			password: "123456",
		},
		{
			password: "correctHorseBatteryStaple",
		},
	}

	for i, c := range cases {
		t.Run(fmt.Sprintf("Test case %v", i), func(t *testing.T) {
			_, err := HashPassword(c.password)
			if err != nil {
				t.Errorf("HashPassword returned an error: %v\n", err)
				return
			}
		})
	}
}

func TestCheckPasswordHash(t *testing.T) {
	cases := []struct {
		password string
		hash     string
	}{
		{
			password: "123456",
			hash:     "$argon2id$v=19$m=65536,t=1,p=16$A5yxxYyZ9GBJFzgaeIHy7w$ccvfBEmlQmYavztPa6Vt8fv4MLDnu3NgOCMufUjyvGI",
		},
		{
			password: "correctHorseBatteryStaple",
			hash:     "$argon2id$v=19$m=65536,t=1,p=16$65kdNiQ5DC7fOaqD5aOC5A$cOBR2kud94LZ6ASfYzud8VaLry4Ts0skXkkgfWhXnt4",
		},
	}

	for i, c := range cases {
		t.Run(fmt.Sprintf("Test case %v", i), func(t *testing.T) {
			pass, err := CheckPasswordHash(c.password, c.hash)
			if err != nil {
				t.Errorf("CheckPassword returned an error: %v\n", err)
				return
			}
			if !pass {
				t.Error("Password and hash didn't match\n")
				return
			}
		})
	}
}

func TestMakeJWT(t *testing.T) {
	cases := []struct {
		tokenSecret string
		expiresIn   time.Duration
		wait        time.Duration
		userIdStr   string
		expectError bool
	}{
		{
			tokenSecret: "omweoimfasioejfsefw0sjfsimcemme0wm0s9mdfc0semfs",
			expiresIn:   time.Hour,
			wait:        time.Millisecond,
			userIdStr:   "7de32306-0902-49e3-8167-4645299f1cbd",
			expectError: false,
		},
		{
			tokenSecret: "omweoimfasioejfsefw0sjfsimcemme0wm0s9mdfc0semfs",
			expiresIn:   time.Millisecond,
			wait:        time.Second,
			userIdStr:   "7de32306-0902-49e3-8167-4645299f1cbd",
			expectError: true,
		},
		{
			tokenSecret: "miosdfiowesmiosmiomfsoimfoismiefmsdoimfsiweio3",
			expiresIn:   time.Hour,
			wait:        time.Millisecond,
			userIdStr:   "7de32306-0902-49e3-8167-4645299f1cbd",
			expectError: true,
		},
	}

	for i, c := range cases {
		t.Run(fmt.Sprintf("Test case %v", i), func(t *testing.T) {
			userId, err := uuid.Parse(c.userIdStr)
			if err != nil {
				t.Errorf("Error parsing User ID: %v", err)
				return
			}
			tokenString, err := MakeJWT(userId, "omweoimfasioejfsefw0sjfsimcemme0wm0s9mdfc0semfs", c.expiresIn)
			if err != nil {
				t.Errorf("Error creating tokenString: %v", err)
				return
			}
			time.Sleep(c.wait)
			actualUserId, err := ValidateJWT(tokenString, c.tokenSecret)
			if c.expectError {
				if err == nil {
					t.Error("Expected token to be rejected")
					return
				}
			} else {
				if err != nil {
					t.Errorf("Token was unexpectedly rejected: %v", err)
					return
				}
				if actualUserId != userId {
					t.Errorf("User ID %v didn't match expected ID %v", actualUserId, userId)
					return
				}
			}
		})
	}
}

func TestGetBearerToken(t *testing.T) {
	cases := []struct {
		authHeader       string
		createAuthHeader bool
		expectError      bool
		tokenString      string
	}{
		{
			authHeader:       "Bearer maofiweiomwaeimf2m230f9we909203mwe09fmse09f23k02m3mdsf09m2",
			createAuthHeader: true,
			expectError:      false,
			tokenString:      "maofiweiomwaeimf2m230f9we909203mwe09fmse09f23k02m3mdsf09m2",
		},
		{
			authHeader:       "Bearer maofiweiomwaeimf2m230f9we909203mwe09fmse09f23k02m3mdsf09m2",
			createAuthHeader: false,
			expectError:      true,
			tokenString:      "",
		},
		{
			authHeader:       "",
			createAuthHeader: true,
			expectError:      true,
			tokenString:      "",
		},
		{
			authHeader:       "ApiKey maofiweiomwaeimf2m230f9we909203mwe09fmse09f23k02m3mdsf09m2",
			createAuthHeader: true,
			expectError:      true,
			tokenString:      "",
		},
	}

	for i, c := range cases {
		t.Run(fmt.Sprintf("Test case %v", i), func(t *testing.T) {
			const AUTH = "Authorization"
			header := make(map[string][]string)
			if c.createAuthHeader {
				header[AUTH] = make([]string, 0)
				header[AUTH] = append(header[AUTH], c.authHeader)
			}
			token, err := GetBearerToken(header)
			if c.expectError {
				if err == nil {
					t.Errorf("Expected error from getting bearer token")
					return
				}
			} else {
				if err != nil {
					t.Errorf("Error getting bearer token: %v", err)
					return
				}
				if token != c.tokenString {
					t.Errorf("Token %s did not match expected %s", token, c.tokenString)
					return
				}
			}
		})
	}
}

func TestGetApiKey(t *testing.T) {
	cases := []struct {
		authHeader       string
		createAuthHeader bool
		expectError      bool
		tokenString      string
	}{
		{
			authHeader:       "ApiKey maofiweiomwaeimf2m230f9we909203mwe09fmse09f23k02m3mdsf09m2",
			createAuthHeader: true,
			expectError:      false,
			tokenString:      "maofiweiomwaeimf2m230f9we909203mwe09fmse09f23k02m3mdsf09m2",
		},
		{
			authHeader:       "ApiKey maofiweiomwaeimf2m230f9we909203mwe09fmse09f23k02m3mdsf09m2",
			createAuthHeader: false,
			expectError:      true,
			tokenString:      "",
		},
		{
			authHeader:       "",
			createAuthHeader: true,
			expectError:      true,
			tokenString:      "",
		},
		{
			authHeader:       "Bearer maofiweiomwaeimf2m230f9we909203mwe09fmse09f23k02m3mdsf09m2",
			createAuthHeader: true,
			expectError:      true,
			tokenString:      "",
		},
	}

	for i, c := range cases {
		t.Run(fmt.Sprintf("Test case %v", i), func(t *testing.T) {
			const AUTH = "Authorization"
			header := make(map[string][]string)
			if c.createAuthHeader {
				header[AUTH] = make([]string, 0)
				header[AUTH] = append(header[AUTH], c.authHeader)
			}
			token, err := GetAPIKey(header)
			if c.expectError {
				if err == nil {
					t.Errorf("Expected error from getting ApiKey token")
					return
				}
			} else {
				if err != nil {
					t.Errorf("Error getting ApiKey token: %v", err)
					return
				}
				if token != c.tokenString {
					t.Errorf("Token %s did not match expected %s", token, c.tokenString)
					return
				}
			}
		})
	}
}
