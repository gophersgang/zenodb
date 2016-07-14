package tdb

import (
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/getlantern/tdb/sql"
	"github.com/getlantern/yaml"
)

type Schema map[string]*TableDef

type TableDef struct {
	HotPeriod       time.Duration
	RetentionPeriod time.Duration
	SQL             string
}

func (db *DB) pollForSchema(filename string) error {
	stat, err := os.Stat(filename)
	if err != nil {
		return err
	}

	err = db.ApplySchemaFromFile(filename)
	if err != nil {
		return err
	}

	go func() {
		for {
			time.Sleep(100 * time.Millisecond)
			newStat, err := os.Stat(filename)
			if err != nil {
				log.Errorf("Unable to stat schema: %v", err)
				continue
			}
			if newStat.ModTime().After(stat.ModTime()) || newStat.Size() != stat.Size() {
				log.Debug("Schema file changed, applying")
				db.ApplySchemaFromFile(filename)
				stat = newStat
			}
		}
	}()

	return nil
}

func (db *DB) ApplySchemaFromFile(filename string) error {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	var schema Schema
	err = yaml.Unmarshal(b, &schema)
	if err != nil {
		return err
	}
	return db.ApplySchema(schema)
}

func (db *DB) ApplySchema(schema Schema) error {
	for name, tdef := range schema {
		name = strings.ToLower(name)
		t := db.getTable(name)
		if t == nil {
			log.Debugf("Creating table %v", name)
			err := db.CreateTable(name, tdef.HotPeriod, tdef.RetentionPeriod, tdef.SQL)
			if err != nil {
				return err
			}
		} else {
			// TODO: support more comprehensive altering of tables (maybe)
			log.Debugf("Cowardly altering where and nothing else on table %v", name)
			q, err := sql.Parse(tdef.SQL)
			if err != nil {
				return err
			}
			t.applyWhere(q.Where)
		}
	}

	return nil
}