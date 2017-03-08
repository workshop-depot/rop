package rop

import (
	"encoding/csv"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strconv"
	"testing"
	"time"
)

//-----------------------------------------------------------------------------

func TestMain(m *testing.M) {
	defer time.Sleep(time.Millisecond * 300)
	createSampleCSV()
	defer deleteSampleCSV()

	m.Run()
}

func TestErrorMessages(t *testing.T) {
	var x error
	_ = x

	{
		var y Error
		x = y
	}
}

func TestSample01Simple(t *testing.T) {
	errBoom := Error(`BOOM!`)

	var step1 = func(input Result) Result {
		return input
	}
	var step2 = func(input Result) Result {
		if input.Res != `2nd` {
			input.AddErr(errBoom)
			return input
		}
		return input
	}
	var step3 = func(input Result) Result {
		if len(input.Err) > 0 {
			// ...
		}
		return input
	}
	c := Chain(step1, step2, step3)

	r := NewResult()
	r.Res = `1st`
	res := c(*r)
	if matchFind(errBoom, res.Err...) < 0 {
		t.Fail()
	}

	r = NewResult()
	r.Res = `2nd`
	res = c(*r)
	if res.Res != `2nd` {
		t.Fail()
	}
}

func matchFind(msg error, list ...error) int {
	if len(list) == 0 {
		return -1
	}
	for k, v := range list {
		if v == msg {
			return k
		}
	}
	return -1
}

func TestSample02Handlers(t *testing.T) {
	add1 := func(input interface{}) (interface{}, error) {
		i, ok := input.(int)
		if !ok {
			return nil, ErrNotInt
		}
		return i + 1, nil
	}
	add2 := func(input interface{}) interface{} {
		i, ok := input.(int)
		if !ok {
			return input
		}
		return i + 2
	}
	checkNegative := func(input interface{}) error {
		i, ok := input.(int)
		if !ok {
			return ErrNotInt
		}
		if i < 0 {
			return ErrNegative
		}
		return nil
	}
	logger := func(interface{}) {
		// logging
	}
	checkOdd := func(input Result) Result {
		i, ok := input.Res.(int)
		if !ok {
			return input
		}
		if i&1 == 1 {
			input.AddMsg(MsgIsOdd)
		}
		return input
	}
	checkEven := func(input Result) Result {
		i, ok := input.Res.(int)
		if !ok {
			return input
		}
		if i&1 != 1 {
			input.AddMsg(MsgIsEven)
		}
		return input
	}
	errInterceptor := func(in Result) Result {
		if matchFind(MsgIsEven, in.Msg...) >= 0 || matchFind(MsgIsOdd, in.Msg...) >= 0 {
			in.AddMsg(MsgProcessed)
		} else {
			in.AddErr(ErrNotProcessed)
		}
		return in
	}

	c := Chain(add1, add2, checkOdd, checkEven, checkNegative, logger, errInterceptor)

	{
		r := NewResult()
		r.Res = `1`
		res := c(*r)
		if matchFind(ErrNotProcessed, res.Err...) < 0 {
			t.Fail()
		}
		if matchFind(ErrNotInt, res.Err...) < 0 {
			t.Fail()
		}
	}

	{
		r := NewResult()
		r.Res = 0
		res := c(*r)
		if matchFind(MsgProcessed, res.Msg...) < 0 {
			t.Fail()
		}
		if matchFind(MsgIsOdd, res.Msg...) < 0 {
			t.Fail()
		}
	}

	{
		r := NewResult()
		r.Res = 1
		res := c(*r)
		if matchFind(MsgProcessed, res.Msg...) < 0 {
			t.Fail()
		}
		if matchFind(MsgIsEven, res.Msg...) < 0 {
			t.Fail()
		}
	}
}

const (
	MsgIsEven    = Error(`MsgIsEven`)
	MsgIsOdd     = Error(`MsgIsOdd`)
	MsgProcessed = Error(`MsgProcessed`)

	ErrNotInt       = Error(`ErrNotInt`)
	ErrNegative     = Error(`ErrNegative`)
	ErrNotProcessed = Error(`ErrNotProcessed`)
)

//-----------------------------------------------------------------------------

func deleteSampleCSV() {
	os.Remove(csvPath)
}

func createSampleCSV() {
	csvPath = fmt.Sprintf("/tmp/TEST_%d.csv", time.Now().UnixNano())
	f, err := os.Create(csvPath)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	rndSrc := rand.NewSource(time.Now().UnixNano())
	rnd := rand.New(rndSrc)
	w := csv.NewWriter(f)
	w.Comma = '|'
	defer w.Flush()

	for i := 0; i < 1000; i++ {
		var sign float64 = -1
		if rnd.Intn(10) >= 5 {
			sign = 1
		}
		name := fmt.Sprintf("LOC_NAME_%v", rnd.Int63n(math.MaxInt64))
		longitude := fmt.Sprintf("%.6f", rnd.Float64()*180*sign)
		latitude := fmt.Sprintf("%.6f", rnd.Float64()*90*sign)
		w.Write([]string{
			name,
			longitude,
			latitude,
		})
	}
}

var (
	csvPath string
)

func TestSample02GeoCSV(t *testing.T) {
	f, err := os.Open(csvPath)
	if err != nil {
		t.Error(err)
		t.Fail()
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.Comma = '|'

	minDist := math.MaxFloat64
	findMinDist := Chain(parse, (&toPair{}).Process, calcDist)
	var record []string
	var readErr error
	for ; readErr == nil; record, readErr = reader.Read() {
		r := NewResult()
		r.Res = record
		res := findMinDist(*r)
		if len(res.Err) > 0 {
			// t.Log(res.Err)
			continue
		}
		d, ok := res.Res.(float64)
		if !ok {
			t.Error(`RESULT SHOULD BE A float64`)
			t.Fail()
		}
		if d < minDist {
			minDist = d
		}
	}
	if minDist == math.MaxFloat64 {
		t.Log(`minDist is max`)
		t.Fail()
	}
}

func calcDist(input Result) Result {
	if input.Res == nil {
		input.AddErr(Error(`NO PAYLOAD`))
		return input
	}
	p, ok := input.Res.(pair)
	if !ok {
		input.AddErr(Error(`PAYLOAD IS NOT A pair`))
		return input
	}
	if p.fst == nil || p.snd == nil {
		input.AddErr(Error(`PAIR MUST CONTAIN TWO LOCATIONS`))
		return input
	}
	d := distance(*p.fst, *p.snd)
	input.Res = d
	return input
}

// this is a statefull processor
type toPair struct {
	fst, snd *location
}

func (x *toPair) Process(input Result) Result {
	if input.Res == nil {
		input.AddErr(Error(`NO PAYLOAD`))
		return input
	}
	loc, ok := input.Res.(location)
	if !ok {
		input.AddErr(Error(`PAYLOAD IS NOT A location`))
		return input
	}
	x.fst, x.snd = x.snd, &loc
	input.Res = pair{x.fst, x.snd}
	return input
}

type pair struct{ fst, snd *location }

func parse(input Result) Result {
	if input.Res == nil {
		input.AddErr(Error(`NO PAYLOAD`))
		return input
	}
	record, ok := input.Res.([]string)
	if !ok {
		input.AddErr(Error(`PAYLOAD IS NOT A []string`))
		return input
	}
	if len(record) != 3 {
		input.AddErr(Error(`RECORD OF []string MUST HAVE 3 ITEMS`))
		return input
	}
	lon, err := strconv.ParseFloat(record[1], 64)
	if err != nil {
		input.AddErr(err)
		return input
	}
	lat, err := strconv.ParseFloat(record[2], 64)
	if err != nil {
		input.AddErr(err)
		return input
	}
	input.Res = location{record[0], lon, lat}
	return input
}

// this calculates cartesian distance, just for demonstration.
// on a spherical surface one should use something like haversine
// https://en.wikipedia.org/wiki/Haversine_formula
// and there are better algorithms for finding min dist and clustering out there!
func distance(loc1, loc2 location) float64 {
	d := math.Pow(loc1.longitude-loc2.longitude, 2)
	d += math.Pow(loc1.latitude-loc2.latitude, 2)
	d = math.Sqrt(d)
	return d
}

type location struct {
	name                string
	longitude, latitude float64
}

//-----------------------------------------------------------------------------

func TestSample02ConcurrentGeoCSV(t *testing.T) {
	f, err := os.Open(csvPath)
	if err != nil {
		t.Error(err)
		t.Fail()
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.Comma = '|'

	in := make(chan Result, 30)

	minDist := math.MaxFloat64
	findMinDist := PipeChain(in, parse, (&toPair{}).Process, calcDist)

	go func() {
		defer close(in)

		var record []string
		var readErr error
		for ; readErr == nil; record, readErr = reader.Read() {
			r := NewResult()
			r.Res = record
			in <- *r
		}
	}()

	for res := range findMinDist {
		if len(res.Err) > 0 {
			// t.Log(`error:`, res.Err)
			continue
		}
		d, ok := res.Res.(float64)
		if !ok {
			t.Error(`RESULT SHOULD BE A float64`)
			t.Fail()
		}
		if d < minDist {
			minDist = d
		}
	}

	if minDist == math.MaxFloat64 {
		t.Logf("%.6f", minDist)
		t.Fail()
	}
}

//-----------------------------------------------------------------------------
