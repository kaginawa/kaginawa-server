package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/gorilla/sessions"
	"github.com/quasoft/memstore"
	"gopkg.in/boj/redistore.v1"
)

const sessionName = "kaginawa-session"

var sessionStore sessions.Store

// Session implements current session operations.
type Session struct {
	instance *sessions.Session
}

func initSession() error {
	gob.Register(map[string]interface{}{})
	secretKey := []byte(oauthConfig.ClientSecret) // use client secret to cookie secret
	if redisURL := os.Getenv("REDIS_URL"); len(redisURL) > 0 {
		ep, err := url.Parse(redisURL)
		if err != nil {
			return fmt.Errorf("failed to parse redis url %s: %v", redisURL, err)
		}
		addr := fmt.Sprintf("%s:%s", ep.Hostname(), ep.Port())
		pw, _ := ep.User.Password()
		store, err := redistore.NewRediStore(5, "tcp", addr, pw, secretKey)
		if err != nil {
			return fmt.Errorf("failed to connect redis %s: %v", redisURL, err)
		}
		sessionStore = store
	} else {
		sessionStore = memstore.NewMemStore(secretKey)
	}
	return nil
}

func getSession(r *http.Request) *Session {
	session, err := sessionStore.Get(r, sessionName)
	if err != nil {
		log.Printf("failed to read session: %v", err)
		return &Session{}
	}
	return &Session{session}
}

func (s *Session) beginAuth(r *http.Request, w http.ResponseWriter) (string, error) {
	if s.instance == nil {
		return "", errors.New("failed to initialize session, you probably need to delete the cookie")
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	state := base64.StdEncoding.EncodeToString(b)
	s.instance.Values["state"] = state
	return state, s.instance.Save(r, w)
}

func (s *Session) state() string {
	if s, ok := s.instance.Values["state"].(string); ok {
		return s
	}
	return ""
}

func (s *Session) endAuth(r *http.Request, w http.ResponseWriter, idToken interface{}, accessToken string,
	profile map[string]interface{}) error {
	s.instance.Values["guest"] = false
	s.instance.Values["id_token"] = idToken
	s.instance.Values["access_token"] = accessToken
	s.instance.Values["profile"] = profile
	return s.instance.Save(r, w)
}

func (s *Session) logout(w http.ResponseWriter, r *http.Request) error {
	s.instance.Values["guest"] = true
	s.instance.Values["profile"] = map[string]interface{}{}
	return s.instance.Save(r, w)
}

func (s *Session) isLoggedIn() bool {
	if s.instance == nil {
		return false
	}
	v, ok := s.instance.Values["guest"]
	return ok && v == false
}

func (s *Session) name() string {
	if v, ok := s.profile()["name"].(string); ok {
		return v
	}
	return ""
}

func (s *Session) email() string {
	if v, ok := s.profile()["email"].(string); ok {
		return v
	}
	return ""
}

func (s *Session) pictureURL(defaultPicture string) string {
	if v, ok := s.profile()["picture"].(string); ok {
		return v
	}
	return defaultPicture
}

func (s *Session) profile() map[string]interface{} {
	if profile, ok := s.instance.Values["profile"].(map[string]interface{}); ok {
		return profile
	}
	return map[string]interface{}{}
}

func (s *Session) getValue(key string) string {
	if v, ok := s.instance.Values[key].(string); ok {
		return v
	}
	return ""
}

func (s *Session) setValue(r *http.Request, w http.ResponseWriter, key, value string) error {
	s.instance.Values[key] = value
	return s.instance.Save(r, w)
}
