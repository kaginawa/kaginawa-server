package kaginawa

import (
	"bytes"
	base32 "encoding/base32"
	"encoding/base64"
	"encoding/gob"
	"errors"
	"fmt"
	"github.com/gorilla/securecookie"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/sessions"
)

// UserSession defines user session attributes.
type UserSession struct {
	ID      string           `bson:"sid"`
	Values  string           `bson:"values"` // encoded map[interface{}]interface{} using gob + base64
	Options sessions.Options `bson:"options"`
	Time    time.Time        `bson:"time" dynamodbav:"-"` // Used by MongoDB (TTL index)
	TTL     int64            `bson:"-"`                   // Used by DynamoDB (TTL attribute)
}

// NewUserSession will creates UserSession object.
func NewUserSession(session sessions.Session, ttl int64) (*UserSession, error) {
	buf := &bytes.Buffer{}
	err := gob.NewEncoder(buf).Encode(session.Values)
	if err != nil {
		return nil, fmt.Errorf("failed to encode data: %w", err)
	}
	encodedValues := base64.StdEncoding.EncodeToString(buf.Bytes())
	return &UserSession{
		ID:      session.ID,
		Values:  encodedValues,
		Options: *session.Options,
		TTL:     ttl,
	}, nil
}

// DecodeValues decodes raw values using gob.
func (s UserSession) DecodeValues() (map[interface{}]interface{}, error) {
	data, err := base64.StdEncoding.DecodeString(s.Values)
	if err != nil {
		return nil, errors.New("failed to decode data")
	}
	values := map[interface{}]interface{}{}
	if err = gob.NewDecoder(bytes.NewReader(data)).Decode(&values); err != nil {
		return nil, errors.New("failed to decode data")
	}
	return values, nil
}

// SessionStore implements gorilla/sessions sessions.Store.
type SessionStore struct {
	options sessions.Options
	db      DB
}

// NewSessionDB constructs a SessionStore instance.
func NewSessionDB(db DB, expireSec int) (*SessionStore, error) {
	store := &SessionStore{db: db}
	store.options.Path = "/"
	store.options.HttpOnly = true
	if expireSec > 0 {
		store.options.MaxAge = expireSec
	}
	return store, nil
}

// Get implements sessions.Store.Get method.
func (s *SessionStore) Get(req *http.Request, name string) (*sessions.Session, error) {
	return sessions.GetRegistry(req).Get(s, name)
}

// New implements sessions.Store.New method.
func (s *SessionStore) New(req *http.Request, name string) (*sessions.Session, error) {
	if cookie, errCookie := req.Cookie(name); errCookie == nil {
		session := sessions.NewSession(s, name)
		err := s.load(cookie.Value, session)
		if err == nil {
			return session, nil
		}
	}
	session := sessions.NewSession(s, name)
	session.ID = strings.TrimRight(base32.StdEncoding.EncodeToString(securecookie.GenerateRandomKey(32)), "=")
	session.IsNew = true
	session.Options = &sessions.Options{
		Path:     s.options.Path,
		Domain:   s.options.Domain,
		MaxAge:   s.options.MaxAge,
		Secure:   s.options.Secure,
		HttpOnly: s.options.HttpOnly,
	}
	return session, nil
}

// Save implements sessions.Store.Save method.
func (s *SessionStore) Save(_ *http.Request, w http.ResponseWriter, session *sessions.Session) error {
	err := s.save(session)
	if err != nil {
		return err
	}
	if session.Options != nil && session.Options.MaxAge < 0 {
		cookie := newCookie(session, session.Name(), "")
		http.SetCookie(w, cookie)
		return s.db.DeleteUserSession(session.ID)
	}
	if !session.IsNew {
		return nil
	}
	cookie := newCookie(session, session.Name(), session.ID)
	http.SetCookie(w, cookie)
	return nil
}

func newCookie(session *sessions.Session, name, value string) *http.Cookie {
	cookie := &http.Cookie{
		Name:  name,
		Value: value,
	}
	if opts := session.Options; opts != nil {
		cookie.Path = opts.Path
		cookie.Domain = opts.Domain
		cookie.MaxAge = opts.MaxAge
		cookie.HttpOnly = opts.HttpOnly
		cookie.Secure = opts.Secure
	}
	return cookie
}

func (s *SessionStore) load(value string, session *sessions.Session) error {
	us, err := s.db.GetUserSession(value)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}
	if us == nil {
		return errors.New("session not found")
	}
	if us.TTL > 0 && us.TTL < time.Now().Unix() {
		return errors.New("session expired")
	}
	session.Values, err = us.DecodeValues()
	if err != nil {
		return fmt.Errorf("failed to decode session value: %w", err)
	}
	session.IsNew = false
	session.ID = us.ID
	session.Options = &us.Options
	return nil
}

func (s *SessionStore) save(session *sessions.Session) error {
	expiresAt := time.Now().Add(time.Duration(session.Options.MaxAge) * time.Second)
	us, err := NewUserSession(*session, expiresAt.Unix())
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}
	return s.db.PutUserSession(*us)
}
