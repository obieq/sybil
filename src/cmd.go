package edb
import "fmt"
import "flag"
import "strings"
import "time"
import "strconv"

var f_RESET = flag.Bool("reset", false, "Reset the DB")
var f_TABLE = flag.String("table", "", "Table to operate on")
var f_OP = flag.String("op", "avg", "metric to calculate, either 'avg' or 'hist'")
var f_ADD_RECORDS = flag.Int("add", 0, "Add data?")
var f_PRINT = flag.Bool("print", false, "Print some records")
var f_PRINT_INFO = flag.Bool("info", false, "Print table info")
var f_INT_FILTERS = flag.String("int-filter", "", "Int filters, format: col:op:val")
var f_STR_FILTERS = flag.String("str-filter", "", "Str filters, format: col:op:val")

var f_SESSION_COL = flag.String("session", "", "Column to use for sessionizing")
var f_INTS = flag.String("int", "", "Integer values to aggregate")
var f_STRS = flag.String("str", "", "String values to load")
var f_GROUPS = flag.String("group", "", "values group by")

var GROUP_BY  []string

func queryTable(name string, loadSpec LoadSpec, querySpec QuerySpec) {
  table := getTable(name)

  lstart := time.Now()
  lend := time.Now()
  fmt.Println("LOADING RECORDS INTO TABLE TOOK", lend.Sub(lstart))

  // TODO: ADD FILTER SPECIFICATIONS
  start := time.Now()
  ret := table.MatchRecords(querySpec.Filters)
  end := time.Now()
  fmt.Println("FILTER RETURNED", len(ret), "RECORDS, TOOK", end.Sub(start))

  table.AggRecords(ret, querySpec)

  if (*f_SESSION_COL != "") {
    start = time.Now()
    session_maps := SessionizeRecords(ret, *f_SESSION_COL)
    end = time.Now()
    fmt.Println("SESSIONIZED", len(ret), "RECORDS INTO", len(session_maps), "SESSIONS, TOOK", end.Sub(start))
  }
}

func ParseCmdLine() {
  flag.Parse()

  fmt.Println("Starting DB")
  fmt.Println("TABLE", *f_TABLE);



  table := *f_TABLE
  if table == "" { table = "test0" }
  t := getTable(table)

  ints := make([]string, 0)
  groups := make([]string, 0)
  strs := make([]string, 0)
  strfilters := make([]string, 0)
  intfilters := make([]string, 0)

  if *f_GROUPS != "" {
    groups = strings.Split(*f_GROUPS, ",")
    GROUP_BY = groups

  }

  if *f_STRS != "" {
    strs = strings.Split(*f_STRS, ",")

  }

  if *f_INTS != "" {
    ints = strings.Split(*f_INTS, ",")
  }

  if *f_INT_FILTERS != "" {
    intfilters = strings.Split(*f_INT_FILTERS, ",")
  }

  if *f_STR_FILTERS != "" {
    strfilters = strings.Split(*f_STR_FILTERS, ",")
  }



  groupings := []Grouping{}
  for _, g := range groups {
    groupings = append(groupings, Grouping{g})
  }

  aggs := []Aggregation {}
  for _, agg := range ints {
    aggs = append(aggs, Aggregation{op: *f_OP, name: agg})
  }

  filters := []Filter{}
  for _, filt := range intfilters {
    tokens := strings.Split(filt, ":")
    col := tokens[0]
    op := tokens[1]
    val, _ := strconv.ParseInt(tokens[2], 10, 64)

    filters = append(filters, t.IntFilter(col, op, int(val)))
  }

  for _, filter := range strfilters {
    tokens := strings.Split(filter, ":")
    col := tokens[0]
    op := tokens[1]
    val := tokens[2]

    filters = append(filters, t.StrFilter(col, op, val))

  }

  querySpec := QuerySpec{Groups: groupings, Filters: filters, Aggregations: aggs }
  punctuateSpec(&querySpec)

  loadSpec := NewLoadSpec()
  for _, v := range groups { loadSpec.Str(v) }
  for _, v := range strs { loadSpec.Str(v) } 
  for _, v := range ints { loadSpec.Int(v) }


  fmt.Println("USING LOAD SPEC", loadSpec)

  fmt.Println("USING QUERY SPEC", querySpec)

  t.LoadRecords(&loadSpec)
  add_records()
  start := time.Now()
  queryTable(table, loadSpec, querySpec)
  end := time.Now()
  fmt.Println("TESTING TABLE TOOK", end.Sub(start))

  start = time.Now()
  SaveTables()
  end = time.Now()
  fmt.Println("SERIALIZED DB TOOK", end.Sub(start))

  if *f_PRINT {
    t := getTable(table)
    count := 0
    for _, b := range t.BlockList {
      for _, r := range b.RecordList {
	count++
	t.PrintRecord(r)
	if count > 10 {
	  break
	}
      }

      if count > 10 {
	break
      }

    }

  }

  if *f_PRINT_INFO {
    t := getTable(table)
    t.PrintColInfo()
  }
}