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
	CreatedAt      time.Time
	Authenticated  bool
	mu             sync.Mutex
}

type Sessions struct {
	mu      sync.RWMutex
	entries map[string]*Session
}

func NewSessions() *Sessions { return &Sessions{entries: make(map[string]*Session)} }

func (s *Sessions) Start(terminal config.Terminal) (*Session, bool, error) {
	salt, err := randomAlphaNumeric(64)
	if err != nil {
		return nil, false, fmt.Errorf("generate salt: %w", err)
	}
	challenge, err := randomAlphaNumeric(64)
	if err != nil {
		return nil, false, fmt.Errorf("generate challenge: %w", err)
	}
	// A fixed, documented value makes the key derivation reproducible for a
	// terminal and avoids negotiating values outside the allowed range.
	session := &Session{Terminal: terminal, Salt: salt, LoginChallenge: challenge, Iterations: 4096, CreatedAt: time.Now().UTC()}
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
	return session, wasAuthenticated, nil
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

func (s *Session) LoginExpired(now time.Time) bool {
	return now.After(s.CreatedAt.Add(time.Duration(s.Terminal.CommandSeconds*3) * time.Second))
}

func (s *Session) IssueNextChallenge() (string, error) {
	challenge, err := randomAlphaNumeric(64)
	if err != nil {
		return "", err
	}
	s.NextChallenge = challenge
	return challenge, nil
}
