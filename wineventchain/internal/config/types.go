package config

type Config struct {
	StateStore struct {
		Type StoreType `env:"TYPE" json:"type"`
		Disk struct {
			Directory string `env:"DIRECTORY" json:"directory"`
		} `envPrefix:"DISK_" json:"disk"`
		Mongo struct {
			ConnectionString string `env:"CONNECTION_STRING" json:"connection_string"`
			DatabaseName     string `env:"DB_NAME" json:"db_name"`
		} `envPrefix:"MONGODB_" json:"mongodb"`
	} `envPrefix:"STATE_STORE_" json:"state_store"`
}

type StoreType string

const (
	StoreTypeDisk    StoreType = "disk"
	StoreTypeMongoDB StoreType = "mongodb"
)

func Default() Config {
	return Config{}
}
