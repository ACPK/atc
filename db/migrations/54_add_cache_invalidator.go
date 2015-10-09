package migrations

import "github.com/BurntSushi/migration"

func AddCacheInvalidator(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		CREATE TABLE cache_invalidator (last_invalidated timestamp NOT NULL DEFAULT 'epoch');
	`)

	if err != nil {
		return err
	}

	return nil
}
