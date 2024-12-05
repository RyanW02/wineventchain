package datastore

type BaseRepository interface {
	LoadLatest() (int64, error)
	LoadVersion(version int64) error
	Rollback()
	Hash() ([]byte, error)
	Save() ([]byte, int64, error)
}
