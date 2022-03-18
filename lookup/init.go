package lookup

type Lookup interface {
	Get(key string) string
}
