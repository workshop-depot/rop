package rop

import (
	"encoding/csv"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func deleteSampleCSV() {
	os.Remove(csvPath)
}

func createSampleCSV() {
	csvPath = filepath.Join(os.TempDir(), fmt.Sprintf("TEST_%d.csv", time.Now().UnixNano()))
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

func TestMain(m *testing.M) {
	defer time.Sleep(time.Millisecond * 300)
	createSampleCSV()
	defer deleteSampleCSV()

	m.Run()
}

func TestInterfaces(t *testing.T) {
	var hf HandlerFunc
	var _ Handler = hf
}

func TestSample01Simple(t *testing.T) {
	errBoom := errors.New("boom")

	var step1 = func(input *Result) *Result {
		return input
	}
	var step2 = func(input *Result) *Result {
		if input.GetValue() != `2nd` {
			return input.AddErr(errBoom)
		}
		return input
	}
	var step3 = func(input *Result) *Result {
		if len(input.Failure) > 0 {
			// ...
		}
		return input
	}
	c := Chain(nil, step1, step2, step3)

	r := NewResult(nil)
	r.SetValue(`1st`)
	res := c(r)
	assert.Contains(t, res.Failure, errBoom)
	assert.Nil(t, r.Success)
	assert.Len(t, r.Failure, 1)

	r = NewResult(nil)
	r.SetValue(`2nd`)
	res = c(r)
	assert.Equal(t, `2nd`, r.GetValue())
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
	logger := func(input *Result) *Result {
		// logging
		return input
	}
	checkOdd := func(input *Result) *Result {
		i, ok := input.GetValue().(int)
		if !ok {
			return input
		}
		if i&1 == 1 {
			input.AddMsg(MsgIsOdd)
		}
		return input
	}
	checkEven := func(input *Result) *Result {
		i, ok := input.GetValue().(int)
		if !ok {
			return input
		}
		if i&1 != 1 {
			input.AddMsg(MsgIsEven)
		}
		return input
	}
	errInterceptor := func(in *Result) *Result {
		if in.Success != nil {
			if matchFind(MsgIsEven, in.Success.Messages...) >= 0 || matchFind(MsgIsOdd, in.Success.Messages...) >= 0 {
				return in.AddMsg(MsgProcessed)
			}
		}
		in.AddErr(ErrNotProcessed)
		return in
	}

	c := Chain(nil, add1,
		add2,
		checkOdd,
		checkEven,
		checkNegative,
		errInterceptor,
		logger)

	{
		r := NewResult(nil)
		r.SetValue(`1`)
		res := c(r)
		assert.Contains(t, res.Failure, ErrNotInt)
	}

	{
		r := NewResult(nil)
		r.SetValue(0)
		res := c(r)
		assert.NotNil(t, res.Success)
		assert.Contains(t, res.Success.Messages, MsgProcessed)
		assert.Contains(t, res.Success.Messages, MsgIsOdd)
	}

	{
		r := NewResult(nil)
		r.SetValue(1)
		res := c(r)
		assert.NotNil(t, res.Success)
		assert.Contains(t, res.Success.Messages, MsgProcessed)
		assert.Contains(t, res.Success.Messages, MsgIsEven)
	}
}

type message struct{ error }

var (
	MsgIsEven    = message{errors.New(`MsgIsEven`)}
	MsgIsOdd     = message{errors.New(`MsgIsOdd`)}
	MsgProcessed = message{errors.New(`MsgProcessed`)}

	ErrNotInt       = errors.New(`ErrNotInt`)
	ErrNegative     = errors.New(`ErrNegative`)
	ErrNotProcessed = errors.New(`ErrNotProcessed`)
)

//-----------------------------------------------------------------------------

func TestSample02GeoCSV(t *testing.T) {
	f, err := os.Open(csvPath)
	assert.Nil(t, err)
	defer f.Close()

	reader := csv.NewReader(f)
	reader.Comma = '|'

	minDist := math.MaxFloat64
	findMinDist := Chain(nil, parse, (&toPair{}).Process, calcDist)
	var record []string
	var readErr error
	for ; readErr == nil; record, readErr = reader.Read() {
		r := NewResult(nil)
		r.SetValue(record)
		res := findMinDist(r)
		// assert.Len(t, res.Failure, 0)
		if len(res.Failure) > 0 {
			continue
		}
		d, ok := res.GetValue().(float64)
		if !ok {
			t.Fatal(`RESULT SHOULD BE A float64`)
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

func calcDist(input *Result) *Result {
	if input.GetValue() == nil {
		input.AddErr(errors.New(`NO PAYLOAD`))
		return input
	}
	p, ok := input.GetValue().(pair)
	if !ok {
		input.AddErr(errors.New(`PAYLOAD IS NOT A pair`))
		return input
	}
	if p.fst == nil || p.snd == nil {
		input.AddErr(errors.New(`PAIR MUST CONTAIN TWO LOCATIONS`))
		return input
	}
	d := distance(*p.fst, *p.snd)
	input.SetValue(d)
	return input
}

// this is a statefull processor
type toPair struct {
	fst, snd *location
}

func (x *toPair) Process(input *Result) *Result {
	if input.GetValue() == nil {
		input.AddErr(errors.New(`NO PAYLOAD`))
		return input
	}
	loc, ok := input.GetValue().(location)
	if !ok {
		input.AddErr(errors.New(`PAYLOAD IS NOT A location`))
		return input
	}
	x.fst, x.snd = x.snd, &loc
	input.SetValue(pair{x.fst, x.snd})
	return input
}

type pair struct{ fst, snd *location }

func parse(input *Result) *Result {
	if input.GetValue() == nil {
		input.AddErr(errors.New(`NO PAYLOAD`))
		return input
	}
	record, ok := input.GetValue().([]string)
	if !ok {
		input.AddErr(errors.New(`PAYLOAD IS NOT A []string`))
		return input
	}
	if len(record) != 3 {
		input.AddErr(errors.New(`RECORD OF []string MUST HAVE 3 ITEMS`))
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
	input.SetValue(location{record[0], lon, lat})
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
		t.Fatal(err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.Comma = '|'

	in := make(chan *Result, 30)

	minDist := math.MaxFloat64
	findMinDist := PipeChain(nil, in, parse, (&toPair{}).Process, calcDist)

	go func() {
		defer close(in)

		var record []string
		var readErr error
		for ; readErr == nil; record, readErr = reader.Read() {
			r := NewResult(record)
			in <- r
		}
	}()

	for res := range findMinDist {
		if len(res.Failure) > 0 {
			// t.Log(`error:`, res.Err)
			continue
		}
		d, ok := res.GetValue().(float64)
		if !ok {
			t.Fatal(`RESULT SHOULD BE A float64`)
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

func TestSupervisorySteps01(t *testing.T) {
	steps := []interface{}{
		func(in *Result) *Result {
			return in.AddMsg(errors.New("START"))
		},
		func(input interface{}) (interface{}, error) {
			return "RES 1", nil
		},
		func(input interface{}) (interface{}, error) {
			assert.Equal(t, "RES 1", input)
			return nil, errors.New("ERR 2")
		},
		func(input interface{}) (interface{}, error) {
			panic("must never get called")
		},
		func(in *Result) *Result {
			return in.AddMsg(errors.New("supervised"))
		},
		func(in *Result) *Result {
			return in.AddMsg(errors.New("END"))
		},
	}

	c := Chain(nil, steps...)

	{
		r := NewResult(nil)
		r.SetValue(1)
		res := c(r)
		_ = res
		// assert.Nil(t, res.Res)
		// assert.Len(t, res.Success.Messages, 3)
		// assert.Equal(t, "START", res.Msg[0].Error())
		// assert.Equal(t, "supervised", res.Msg[1].Error())
		// assert.Equal(t, "END", res.Msg[2].Error())
		assert.Len(t, res.Failure, 1)
		assert.Equal(t, "ERR 2", res.Failure[0].Error())
	}
}

func TestSupervisorySteps02(t *testing.T) {
	steps := []interface{}{
		func(next Handler) Handler {
			return HandlerFunc(func(r *Result, w ResultWriter) {
				defer func() {
					if e := recover(); e != nil {
						r.AddErr(fmt.Errorf("RECOVERED"))
					}
				}()
				if next != nil {
					next.Handle(w.Last(), w)
				}
			})
		},
		func(in *Result) *Result {
			panic("N/A")
			// return in.AddMsg(errors.New("START"))
		},
		func(input interface{}) (interface{}, error) {
			return "RES 1", nil
		},
		func(input interface{}) (interface{}, error) {
			assert.Equal(t, "RES 1", input)
			return nil, errors.New("ERR 2")
		},
		func(input interface{}) (interface{}, error) {
			panic("must never get called")
		},
		func(in *Result) *Result {
			return in.AddMsg(errors.New("supervised"))
		},
		func(in *Result) *Result {
			return in.AddMsg(errors.New("END"))
		},
	}

	c := Chain(nil, steps...)

	{
		r := NewResult(nil)
		r.SetValue(1)
		res := c(r)
		_ = res
		assert.Len(t, res.Failure, 1)
		assert.Equal(t, "RECOVERED", res.Failure[0].Error())
	}
}
