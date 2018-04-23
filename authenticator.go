package chatkit

import (
	"sync"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
)

// Role is a type used by the role methods to define a role
type AuthenticationResponse struct {
	Status  int               `json:"status"`  // (int| required): Suggested response HTTP status code.
	Headers map[string]string `json:"headers"` // (headers| required): Suggested response headers.
	Body    interface{}       `json:"body"`    // (interface| required): Suggested response body.
}

type ErrorBody struct {
	ErrorType        string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
	ErrorURI         string `json:"error_uri,omitempty"`
}

func (e *ErrorBody) Error() string {
	return e.ErrorDescription
}

type TokenBody struct {
	AccessToken string  `json:"access_token"`
	TokenType   string  `json:"token_type"`
	ExpiresIn   float64 `json:"expires_in"`
}

func NewChatkitToken(
	instanceID string,
	keyID string,
	keySecret string,
	userID *string,
	su bool,
	expiryDuration time.Duration,
) (tokenBody *TokenBody, errBody *ErrorBody) {
	jwtClaims := getGenericTokenClaims(
		instanceID,
		keyID,
		expiryDuration,
	)

	jwtClaims["su"] = su
	if userID != nil {
		jwtClaims["sub"] = userID
	}

	tokenString, err := signToken(keySecret, jwtClaims)

	if err != nil {
		return nil, &ErrorBody{
			ErrorType:        "token_provider/token_signing_failure",
			ErrorDescription: "There was an error signing the token",
		}
	}

	return &TokenBody{
			AccessToken: tokenString,
			TokenType:   "bearer",
			ExpiresIn:   expiryDuration.Seconds(),
		},
		nil
}

func getGenericTokenClaims(
	instanceID string,
	keyID string,
	expiryDuration time.Duration,
) (jwtClaims jwt.MapClaims) {
	timeNow := time.Now()
	tokenExpiry := timeNow.Add(expiryDuration)

	jwtClaims = jwt.MapClaims{
		"instance": instanceID,
		"iss":      "api_keys/" + keyID,
		"iat":      timeNow.Unix(),
		"exp":      tokenExpiry.Unix(),
	}

	return jwtClaims
}

func signToken(
	keySecret string,
	jwtClaims jwt.MapClaims,
) (tokenString string, err error) {
	// Create a new access token object, specifying signing method and the
	// claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtClaims)

	// Sign using the keySecret and get the complete encoded token as a string
	tokenString, err = token.SignedString([]byte(keySecret))
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

type Authenticator interface {
	getSUToken() (string, error)
	authenticate(userID string) AuthenticationResponse
}

type authenticator struct {
	tokenExpiry time.Time
	token       string
	mutex       sync.Mutex

	instanceID string
	keyID      string
	keySecret  string
}

func newAuthenticator(
	instanceID string,
	keyID string,
	keySecret string,
) Authenticator {
	return &authenticator{
		tokenExpiry: time.Now().Add(-time.Minute),
		mutex:       sync.Mutex{},

		instanceID: instanceID,
		keyID:      keyID,
		keySecret:  keySecret,
	}
}

func (auth *authenticator) authenticate(userID string) AuthenticationResponse {
	tokenBody, errorBody := NewChatkitToken(
		auth.instanceID,
		auth.keyID,
		auth.keySecret,
		&userID,
		false,
		time.Hour*24,
	)

	if errorBody != nil {
		return AuthenticationResponse{
			Status:  500,
			Headers: map[string]string{},
			Body:    errorBody,
		}
	}

	return AuthenticationResponse{
		Status:  200,
		Headers: map[string]string{},
		Body:    tokenBody,
	}
}

func (auth *authenticator) getSUToken() (string, error) {
	auth.mutex.Lock()
	defer auth.mutex.Unlock()

	timeNow := time.Now()

	if timeNow.After(auth.tokenExpiry) {
		tokenBody, errorBody := NewChatkitToken(
			auth.instanceID,
			auth.keyID,
			auth.keySecret,
			nil,
			true,
			time.Hour*24,
		)
		if errorBody != nil {
			return "", errorBody
		}
		auth.tokenExpiry = timeNow.Add(time.Hour * 24)
		auth.token = tokenBody.AccessToken
	}
	return auth.token, nil
}
