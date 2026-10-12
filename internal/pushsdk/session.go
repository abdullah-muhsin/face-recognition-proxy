package pushsdk

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/itplus/pushsdk-gateway/internal/config"
	"github.com/itplus/pushsdk-gateway/internal/store"
)

type Session struct {
	Terminal        config.Terminal
	Salt            string
	LoginChallenge  string
	NextChallenge   string
	Iterations      int
	PayloadMode     PayloadMode
	CreatedAt       time.Time
	NextChallengeAt time.Time
	Authenticated   bool
	Online          bool // Process-local liveness; authentication survives idle periods.
	mu              sync.Mutex
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
	Session               *Session
	Previous              *Session
	ReplacedOnlineSession bool
}

const keyDerivationIterations = 4096

// This is a liveness threshold, not a credential expiration. The latest
// authenticated challenge remains valid until used, replaced by AuthInfo,
// revoked by Logout, or invalidated by a terminal configuration change.
const sessionIdleTimeout = time.Minute

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
	wasOnline := false
	if previous != nil {
		previous.mu.Lock()
		wasOnline = previous.Online
		previous.mu.Unlock()
	}
	return SessionStart{Session: session, Previous: previous, ReplacedOnlineSession: wasOnline}, nil
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

func (s *Sessions) Replace(pushSDKSerial string, session *Session) {
	s.mu.Lock()
	if session == nil {
		delete(s.entries, pushSDKSerial)
	} else {
		s.entries[pushSDKSerial] = session
	}
	s.mu.Unlock()
}

// Restore retains configuration-matched authentication regardless of inactivity.
// Restored sessions start offline until a valid request proves device contact.
// Incomplete login challenges still expire at their original deadline.
func (s *Sessions) Restore(ctx context.Context, cfg config.Config, data interface {
	PushSDKSessions(context.Context) ([]store.PushSDKSession, error)
	DeletePushSDKSession(context.Context, string) error
}, now time.Time) (int, error) {
	storedSessions, err := data.PushSDKSessions(ctx)
	if err != nil {
		return 0, err
	}
	restored := 0
	for _, stored := range storedSessions {
		terminal, found := cfg.TerminalBySerialNumber(stored.TerminalSerialNumber)
		if !found || stored.ConfigurationFingerprint != terminal.SessionFingerprint() || !validStoredSession(stored) {
			if err := data.DeletePushSDKSession(ctx, stored.TerminalSerialNumber); err != nil {
				return 0, err
			}
			continue
		}
		session := &Session{
			Terminal:       terminal,
			Salt:           stored.Salt,
			LoginChallenge: stored.LoginChallenge,
			NextChallenge:  stored.NextChallenge,
			Iterations:     stored.Iterations,
			PayloadMode:    PayloadMode(stored.PayloadMode),
			CreatedAt:      stored.CreatedAt,
			Authenticated:  stored.Authenticated,
		}
		if stored.NextChallengeAt != nil {
			session.NextChallengeAt = *stored.NextChallengeAt
		}
		if !session.Authenticated && session.LoginChallengeExpired(now) {
			if err := data.DeletePushSDKSession(ctx, stored.TerminalSerialNumber); err != nil {
				return 0, err
			}
			continue
		}
		s.mu.Lock()
		s.entries[terminal.PushSDKSerial] = session
		s.mu.Unlock()
		if session.Authenticated {
			restored++
		}
	}
	return restored, nil
}

func (s *Session) LoginChallengeExpired(now time.Time) bool {
	return !now.Before(s.CreatedAt.Add(time.Duration(s.Terminal.CommandIntervalSeconds*3) * time.Second))
}

func (s *Session) IsIdle(now time.Time) bool {
	return s.NextChallengeAt.IsZero() || !now.Before(s.NextChallengeAt.Add(sessionIdleTimeout))
}

func (s *Session) IssueNextChallenge() (string, error) {
	challenge, err := randomCustomChallenge()
	if err != nil {
		return "", err
	}
	s.NextChallenge = challenge
	s.NextChallengeAt = time.Now().UTC()
	return challenge, nil
}

func (s *Session) PersistentState() store.PushSDKSession {
	state := store.PushSDKSession{
		TerminalSerialNumber:     s.Terminal.SerialNumber,
		ConfigurationFingerprint: s.Terminal.SessionFingerprint(),
		PayloadMode:              string(s.PayloadMode),
		Salt:                     s.Salt,
		LoginChallenge:           s.LoginChallenge,
		NextChallenge:            s.NextChallenge,
		Iterations:               s.Iterations,
		CreatedAt:                s.CreatedAt,
		Authenticated:            s.Authenticated,
	}
	if !s.NextChallengeAt.IsZero() {
		nextChallengeAt := s.NextChallengeAt
		state.NextChallengeAt = &nextChallengeAt
	}
	return state
}

func validStoredSession(session store.PushSDKSession) bool {
	if !lowerHex64.MatchString(session.ConfigurationFingerprint) || (session.PayloadMode != string(PlaintextPayload) && session.PayloadMode != string(EncryptedPayload)) || !alphaNumeric64.MatchString(session.Salt) || !alphaNumeric64.MatchString(session.LoginChallenge) || session.Iterations != keyDerivationIterations {
		return false
	}
	if !session.Authenticated {
		return session.NextChallenge == "" && session.NextChallengeAt == nil
	}
	return lowerHex64.MatchString(session.NextChallenge) && session.NextChallengeAt != nil
}
