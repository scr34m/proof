package config

const (
	PW_SALT_BYTES   = 32
	SESSION_NAME    = "PROOFSESS"
	VERSION         = "0.2"
	COOKIE_KEY_AUTH = "auth"
	REDIS_CHANNEL   = "events"
)

type AuthUser struct {
	Name     string
	Password string
	Email    string
	Enabled  bool
}

type AuthSite struct {
	Name     string
	Username string
	Password string
	Enabled  bool
}

type AuthConfig struct {
	User []AuthUser
	Site []AuthSite
}
