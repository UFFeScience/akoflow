package connector

type Connection struct {
	URI string
}

func (c Connection) Connect() string {
	return c.URI
}

func New(uri string) Connection {
	return Connection{URI: uri}
}
