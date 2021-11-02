module github.com/kaginawa/kaginawa-server

// +heroku goVersion go1.15

go 1.15

require (
	github.com/aws/aws-sdk-go v1.41.14
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/securecookie v1.1.1
	github.com/gorilla/sessions v1.2.1
	go.mongodb.org/mongo-driver v1.7.2
	golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
)
