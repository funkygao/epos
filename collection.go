package epos

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/peterbourgon/diskv"
	"io"
	"log"
	"os"
	"path/filepath"
)

type Collection struct {
	store     *diskv.Diskv
	indexpath string
	indexes   map[string]*index
}

type Id int64

func transformFunc(s string) []string {
	// special case for internal data
	if s == "_next_id" {
		return []string{}
	}

	data := ""
	if len(s) < 4 {
		data = fmt.Sprintf("%04s", s)
	} else {
		data = s[len(s)-4:]
	}

	return []string{data[2:4], data[0:2]}
}

func (db *Database) openColl(name string) *Collection {
	// create/open collection
	coll := &Collection{store: diskv.New(diskv.Options{
		BasePath:     db.path + "/colls/" + name,
		Transform:    transformFunc,
		CacheSizeMax: 0, // no cache
	}), indexpath: db.path + "/indexes/" + name, indexes: make(map[string]*index}

	os.Mkdir(coll.indexpath, 0755)

	coll.loadIndexes()

	// if _next_id is unset, then set it to 1.
	if _, err := coll.store.Read("_next_id"); err != nil {
		coll.setNextId(Id(1))
	}
	return coll
}

func (c *Collection) loadIndexes() {
	filepath.Walk(c.indexpath, func(path string, info os.FileInfo, err error) error {
		if (info.Mode() & os.ModeType) == 0 {
			if err := c.loadIndex(path, filepath.Base(path)); err != nil {
				log.Printf("loadIndex %s failed: %v", path, err)
				// TODO: should we maybe remove or rebuild index?
			}
		}
		return nil
	})
}

func (c *Collection) loadIndex(filepath, field string) error {
	file, err := os.Open(filepath)
	if err != nil {
		return err
	}

	idx := newIndex(file, field)

	for {
		fpos, _ := file.Seek(0, os.SEEK_CUR)
		var entry indexEntry
		_, err = entry.ReadFrom(file)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if !entry.Deleted() {
			entry.fpos = fpos
			idx.Add(entry)
		}
	}

	c.indexes[field] = idx
	return nil
}

func (c *Collection) setNextId(next_id Id) {
	next_id_buf := make([]byte, binary.MaxVarintLen64)
	length := binary.PutVarint(next_id_buf, int64(next_id))
	c.store.Write("_next_id", next_id_buf[:length])
}

func (c *Collection) getNextId() Id {
	data, _ := c.store.Read("_next_id")
	next_id, _ := binary.Varint(data)
	c.setNextId(Id(next_id + 1))
	return Id(next_id)
}

func (c *Collection) Insert(value interface{}) (Id, error) {
	jsondata, err := json.Marshal(value)
	if err != nil {
		return Id(0), err
	}

	id := c.getNextId()
	err = c.store.Write(fmt.Sprintf("%d", id), jsondata)
	if err != nil {
		c.setNextId(id) // roll back generated ID
		id = Id(0)      // set id to invalid value
	}
	return id, err
}

func (c *Collection) Update(id Id, value interface{}) error {
	jsondata, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return c.store.Write(fmt.Sprintf("%d", id), jsondata)
}

func (c *Collection) AddIndex(field string) error {
	return errors.New("adding index failed")
}

func (c *Collection) RemoveIndex(field string) error {
	delete(c.indexes, field)
	if err := os.Remove(c.indexpath + "/" + field); err != nil {
		return err
	}
	return nil
}

func (c *Collection) Reindex(field string) error {
	if err := c.RemoveIndex(field); err != nil {
		return err
	}
	return c.AddIndex(field)
}

func (c *Collection) Delete(id Id) error {
	return c.store.Erase(fmt.Sprintf("%d", id))
}

func (c *Collection) Query(q Condition) (*Result, error) {
	return nil, errors.New("query failed")
}

func (c *Collection) QueryAll() (*Result, error) {
	return c.Query(&True{})
}

func (c *Collection) Vacuum() error {
	// TODO: clean up indexes
	return nil
}
