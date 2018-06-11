package db

type DB interface {
	GetKeys(user string) (string, string, error)
	SetKeys(user, access, secret string) error
	Close()
}
