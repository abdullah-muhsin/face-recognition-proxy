package pushsdk

import (
	"fmt"
	"sync"
	"time"

	"github.com/itplus/pushsdk-gateway/internal/config"
)

type Session struct {
	Terminal       config.Terminal
	Salt           string
	LoginChallenge string
	NextChallenge  string
	Iterations     int
	PayloadMode    PayloadMode
	CreatedAt      time.Time
	Authenticated  bool
	mu             sync.Mutex
}

// PayloadMode is negotiated once by AuthInfo and applies to every subsequent
// request and response in the session.
type PayloadMode string

const (
	PlaintextPayload PayloadMode = "plaintext"
	EncryptedPayload PayloadMode = "encrypted"
)

func (m PayloadMode) IsEncrypted() bool { return m == EncryptedPayload }

type Sessions struct {
	mu      sync.RWMutex
	entries map[string]*Session
}

func NewSessions() *Sessions { return &Sessions{entries: make(map[string]*Session)} }

type SessionStart struct {
	Session                      *Session
	ReplacedAuthenticatedSession bool
}

const keyDerivationIterations = 4096

func (s *Sessions) Start(terminal config.Terminal, mode PayloadMode) (SessionStart, error) {
	salt, err := randomAlphaNumeric(64)
	if err != nil {
		return SessionStart{}, fmt.Errorf("generate salt: %w", err)
	}
	challenge, err := randomAlphaNumeric(64)
	if err != nil {
		return SessionStart{}, fmt.Errorf("generate challenge: %w", err)
	}
	// This fixed gateway value is inside the vendor's documented 500..5000
	// range; it is deliberately not negotiated from untrusted terminal input.
	session := &Session{Terminal: terminal, Salt: salt, LoginChallenge: challenge, Iterations: keyDerivationIterations, PayloadMode: mode, CreatedAt: time.Now().UTC()}
	s.mu.Lock()
	previous := s.entries[terminal.PushSDKSerial]
	s.entries[terminal.PushSDKSerial] = session
	s.mu.Unlock()
	wasAuthenticated := false
	if previous != nil {
		previous.mu.Lock()
		wasAuthenticated = previous.Authenticated
		previous.mu.Unlock()
	}
	return SessionStart{Session: session, ReplacedAuthenticatedSession: wasAuthenticated}, nil
}

func (s *Sessions) Get(pushSDKSerial string) (*Session, bool) {
	s.mu.RLock()
	session, found := s.entries[pushSDKSerial]
	s.mu.RUnlock()
	return session, found
}

func (s *Sessions) Remove(pushSDKSerial string) {
	s.mu.Lock()
	delete(s.entries, pushSDKSerial)
	s.mu.Unlock()
}

func (s *Session) LoginChallengeExpired(now time.Time) bool {
	return now.After(s.CreatedAt.Add(time.Duration(s.Terminal.CommandIntervalSeconds*3) * time.Second))
}

func (s *Session) IssueNextChallenge() (string, error) {
	challenge, err := randomCustomChallenge()
	if err != nil {
		return "", err
	}
	s.NextChallenge = challenge
	return challenge, nil
}
