module github.com/kaginawa/kaginawa-server

// +heroku goVersion go1.15

go 1.15

require (
	github.com/aws/aws-sdk-go v1.42.25
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/securecookie v1.1.1
	github.com/gorilla/sessions v1.2.1
	go.mongodb.org/mongo-driver v1.8.1
	golang.org/x/crypto v0.0.0-20201216223049-8b5274cf687f
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
)
