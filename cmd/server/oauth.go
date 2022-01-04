package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"

	"golang.org/x/oauth2"
)

var (
	oauthURL    string
	userInfoURL string
	logoutURL   string
	selfURL     string
	oauthConfig *oauth2.Config
)

func initOAuth() error {
	domain := getEnvs("OAUTH_DOMAIN", "AUTH0_DOMAIN")
	if len(domain) == 0 {
		return errors.New("please set $OAUTH_DOMAIN")
	}
	clientID := getEnvs("OAUTH_CLIENT_ID", "AUTH0_CLIENT_ID")
	if len(clientID) == 0 {
		return errors.New("please set $OAUTH_CLIENT_ID")
	}
	clientSecret := getEnvs("OAUTH_CLIENT_SECRET", "AUTH0_CLIENT_SECRET")
	if len(clientSecret) == 0 {
		return errors.New("please set $OAUTH_CLIENT_SECRET")
	}
	selfURL = getEnvs("SELF_URL")
	if len(selfURL) == 0 {
		return errors.New("please set $SELF_URL")
	}
	oauthURL = "https://" + domain
	userInfoURL = oauthURL + "/userinfo"
	logoutURL = oauthURL + "/v2/logout"
	authURL := oauthURL + "/authorize"
	if authURLOverride := getEnvs("OAUTH_AUTH_URL"); len(authURLOverride) > 0 {
		authURL = authURLOverride
	}
	tokenURL := oauthURL + "/oauth/token"
	if tokenURLOverride := getEnvs("OAUTH_TOKEN_URL"); len(tokenURLOverride) > 0 {
		tokenURL = tokenURLOverride
	}
	oauthConfig = &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  selfURL + "/callback",
		Scopes:       []string{"openid", "profile", "email"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  authURL,
			TokenURL: tokenURL,
		},
	}
	return nil
}

func handleOAuthLogin(w http.ResponseWriter, r *http.Request) {
	session := getSession(r)
	if session.instance == nil {
		http.Error(w, "please remove the cookie", http.StatusUnauthorized)
		return
	}
	state, err := session.beginAuth(r, w)
	if err != nil {
		http.Error(w, "failed to start authentication: "+err.Error(), http.StatusUnauthorized)
		return
	}
	audience := oauth2.SetAuthURLParam("audience", userInfoURL)
	http.Redirect(w, r, oauthConfig.AuthCodeURL(state, audience), http.StatusTemporaryRedirect)
}

func handleOAuthLogout(w http.ResponseWriter, r *http.Request) {
	session := getSession(r)
	if err := session.logout(w, r); err != nil {
		http.Error(w, "failed to clear session: "+err.Error(), http.StatusUnauthorized)
		return
	}
	parsed, err := url.Parse(logoutURL)
	if err != nil {
		http.Error(w, "failed to parse logout url: "+err.Error(), http.StatusInternalServerError)
		return
	}
	parameters := url.Values{
		"returnTo":  []string{selfURL + "/logout-complete"},
		"client_id": []string{oauthConfig.ClientID},
	}
	parsed.RawQuery = parameters.Encode()
	http.Redirect(w, r, parsed.String(), http.StatusTemporaryRedirect)
}

func handleOAuthLoginCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	session := getSession(r)
	if state != session.state() {
		http.Error(w, "invalid state parameter", http.StatusUnauthorized)
		return
	}
	code := r.URL.Query().Get("code")
	token, err := oauthConfig.Exchange(context.Background(), code)
	if err != nil {
		if err := session.setValue(r, w, "error", err.Error()); err != nil {
			log.Printf("failed to save session : %v", err)
		}
		handleOAuthLogout(w, r)
		return
	}

	// Getting now the userInfo
	client := oauthConfig.Client(context.Background(), token)
	resp, err := client.Get(userInfoURL)
	if err != nil {
		http.Error(w, "failed to get user info: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer safeClose(resp.Body, "callback body")
	var profile map[string]interface{}
	if err = json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		http.Error(w, "failed to decode profile: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := session.endAuth(r, w, token.Extra("id_token"), token.AccessToken, profile); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func handleOAuthLogoutComplete(w http.ResponseWriter, r *http.Request) {
	session := getSession(r)
	if session.isLoggedIn() {
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}
	errorMessage := session.getValue("error")
	if len(errorMessage) > 0 {
		if err := session.setValue(r, w, "error", ""); err != nil {
			log.Printf("failed to save session: %v", err)
		}
	}
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}
