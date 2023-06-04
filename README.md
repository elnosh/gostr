# gostr

a [nostr](https://github.com/nostr-protocol/nostr) relay written in go

## requirements
* go
* postgres
* docker and docker compose if you don't want to install postgres

## how to run

1. Clone the project

2. Install postgres or with docker with something like [this](https://github.com/docker/awesome-compose/tree/master/postgresql-pgadmin) and do `docker compose up -d`

3. Put the values in the `config.json` file:
```
{
	"host": "{HOST}",
	"user": "{POSTGRES_USER}",
	"password": "{POSTGRES_PW}",
	"dbname": "{DB_NAME}"
}
```

4. With [go](https://go.dev/doc/install) installed, run `go build .` 

5. Run it `./gostr`