module github.com/kaginawa/kaginawa-server

// +heroku goVersion go1.13

go 1.13

require (
	github.com/aws/aws-sdk-go v1.31.11
	github.com/garyburd/redigo v1.6.0 // indirect
	github.com/gorilla/mux v1.7.4
	github.com/gorilla/sessions v1.2.0
	github.com/quasoft/memstore v0.0.0-20191010062613-2bce066d2b0b
	go.mongodb.org/mongo-driver v1.3.0
	golang.org/x/crypto v0.0.0-20190530122614-20be4c3c3ed5
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	gopkg.in/boj/redistore.v1 v1.0.0-20160128113310-fc113767cd6b
)
