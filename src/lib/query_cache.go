package sybil

import "encoding/gob"

import "bytes"
import "fmt"
import "path"
import "io/ioutil"
import "os"
import "compress/gzip"

func init() {
	registerTypesForQueryCache()
}

// this registration is used for saving and decoding cached per block query
// results
func registerTypesForQueryCache() {
	gob.Register(IntFilter{})
	gob.Register(StrFilter{})
	gob.Register(SetFilter{})

	gob.Register(IntField(0))
	gob.Register(StrField(0))
	gob.Register(SetField{})
	gob.Register(&HistCompat{})
	gob.Register(&MultiHistCompat{})
}

func (t *Table) getCachedQueryForBlock(dirname string, querySpec *QuerySpec) (*TableBlock, *QuerySpec) {

	if FLAGS.CACHED_QUERIES == false {
		return nil, nil
	}

	tb := newTableBlock()
	tb.Name = dirname
	tb.table = t
	info := t.LoadBlockInfo(dirname)

	if info == nil {
		Debug("NO INFO FOR", dirname)
		return nil, nil
	}

	if info.NumRecords <= 0 {
		Debug("NO RECORDS FOR", dirname)
		return nil, nil
	}

	tb.Info = info

	blockQuery := CopyQuerySpec(querySpec)
	if blockQuery.LoadCachedResults(tb.Name) {
		t.block_m.Lock()
		t.BlockList[dirname] = &tb
		t.block_m.Unlock()

		return &tb, blockQuery

	}

	return nil, nil

}

// for a per block query cache, we exclude any trivial filters (that are true
// for all records in the block) when creating our cache key
func (querySpec *QuerySpec) GetCacheRelevantFilters(blockname string) []Filter {

	filters := make([]Filter, 0)
	if querySpec == nil {
		return filters
	}

	t := querySpec.Table

	info := t.LoadBlockInfo(blockname)

	if info == nil {
		return filters
	}

	max_record := Record{Ints: IntArr{}, Strs: StrArr{}}
	min_record := Record{Ints: IntArr{}, Strs: StrArr{}}

	if len(info.IntInfoMap) == 0 {
		return filters
	}

	for field_name, _ := range info.StrInfoMap {
		field_id := t.get_key_id(field_name)
		min_record.ResizeFields(field_id)
		max_record.ResizeFields(field_id)
	}

	for field_name, field_info := range info.IntInfoMap {
		field_id := t.get_key_id(field_name)
		min_record.ResizeFields(field_id)
		max_record.ResizeFields(field_id)

		min_record.Ints[field_id] = IntField(field_info.Min)
		max_record.Ints[field_id] = IntField(field_info.Max)

		min_record.Populated[field_id] = INT_VAL
		max_record.Populated[field_id] = INT_VAL
	}

	for _, f := range querySpec.Filters {
		// make the minima record and the maxima records...
		switch fil := f.(type) {
		case IntFilter:
			// we only use block extents for skipping gt and lt filters
			if fil.Op != "lt" && fil.Op != "gt" {
				filters = append(filters, f)
				continue
			}

			if f.Filter(&min_record) && f.Filter(&max_record) {
			} else {
				filters = append(filters, f)
			}

		default:
			filters = append(filters, f)
		}
	}

	return filters

}

func (qs *QuerySpec) GetCacheStruct(blockname string) QueryParams {
	cache_spec := QueryParams(qs.QueryParams)

	// kick out trivial filters
	cache_spec.Filters = qs.GetCacheRelevantFilters(blockname)

	return cache_spec
}

func (qs *QuerySpec) GetCacheKey(blockname string) string {
	return qs.GetCacheStruct(blockname).cacheKey()
}

func (qs *QuerySpec) LoadCachedResults(blockname string) bool {
	if FLAGS.CACHED_QUERIES == false {
		return false
	}

	if FLAGS.SAMPLES {
		return false

	}

	cache_key := qs.GetCacheKey(blockname)

	cache_dir := path.Join(blockname, "cache")
	cache_name := fmt.Sprintf("%s.db", cache_key)
	filename := path.Join(cache_dir, cache_name)

	cachedSpec := QueryResults{}
	err := decodeInto(filename, &cachedSpec)

	if err != nil {
		return false
	}

	qs.QueryResults = cachedSpec

	return true
}

func (qs *QuerySpec) SaveCachedResults(blockname string) {
	if FLAGS.CACHED_QUERIES == false {
		return
	}

	if FLAGS.SAMPLES {
		return
	}

	info := qs.Table.LoadBlockInfo(blockname)

	if info.NumRecords < int32(CHUNK_SIZE) {
		return
	}

	cache_key := qs.GetCacheKey(blockname)

	cachedInfo := qs.QueryResults

	cache_dir := path.Join(blockname, "cache")
	err := os.MkdirAll(cache_dir, 0777)
	if err != nil {
		Debug("COULDNT CREATE CACHE DIR", err, "NOT CACHING QUERY")
		return
	}

	cache_name := fmt.Sprintf("%s.db.gz", cache_key)
	filename := path.Join(cache_dir, cache_name)
	tempfile, err := ioutil.TempFile(cache_dir, cache_name)
	if err != nil {
		Debug("TEMPFILE ERROR", err)
	}

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err = enc.Encode(cachedInfo)

	var gbuf bytes.Buffer
	w := gzip.NewWriter(&gbuf)
	w.Write(buf.Bytes())
	w.Close() // You must close this first to flush the bytes to the buffer.

	if err != nil {
		Warn("cached query encoding error:", err)
		tempfile.Close()
		return
	}

	if err != nil {
		Warn("ERROR CREATING TEMP FILE FOR QUERY CACHED INFO", err)
		tempfile.Close()
		return
	}

	_, err = gbuf.WriteTo(tempfile)
	tempfile.Close()
	if err != nil {
		Warn("ERROR SAVING QUERY CACHED INFO INTO TEMPFILE", err)
		return
	}

	err = RenameAndMod(tempfile.Name(), filename)
	if err != nil {
		Warn("ERROR RENAMING", tempfile.Name())
	}

	return

}
