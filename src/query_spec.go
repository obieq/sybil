package edb
import "sync"

type QuerySpec struct {
  Filters []Filter
  Groups []Grouping
  Aggregations []Aggregation
  Results map[string]*Result
  BlockList map[string]TableBlock

  m *sync.Mutex
  c *sync.Mutex
}

type Filter interface {
  Filter(*Record) bool;
}

type Grouping struct {
  name string
}

type Aggregation struct {
  op string
  name string
}

type Result struct {
  Ints map[string]float64
  Strs map[string]string
  Sets map[string][]string
  Hists map[string]*Hist
}

func punctuateSpec(querySpec *QuerySpec) {
  querySpec.Results = make(map[string]*Result)
  querySpec.m = &sync.Mutex{}
  querySpec.c = &sync.Mutex{}
}
