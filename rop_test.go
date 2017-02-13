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

//-----------------------------------------------------------------------------

func TestSample01Simple(t *testing.T) {
	errBoom := Error(`BOOM!`)

	var step1 ProcessorFunc = func(input Payload) Payload {
		return input
	}
	var step2 ProcessorFunc = func(input Payload) Payload {
		if input.Payload != `2nd` {
			return Payload{Err: errBoom}
		}
		return input
	}
	var step3 ProcessorFunc = func(input Payload) Payload {
		if input.Err != nil {
			return Payload{Err: input.Err}
		}
		return input
	}
	c := Chain(step1, step2, step3)

	res := c.Process(Payload{Payload: `1st`})
	if res.Err != errBoom {
		t.Fail()
	}

	res = c.Process(Payload{Payload: `2nd`})
	if res.Payload != `2nd` {
		t.Fail()
	}
}

//-----------------------------------------------------------------------------

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
	findMinDist := Chain(ProcessorFunc(parse), &toPair{}, ProcessorFunc(calcDist))
	var record []string
	var readErr error
	for ; readErr == nil; record, readErr = reader.Read() {
		res := findMinDist.Process(Payload{Payload: record})
		if res.Err != nil {
			// t.Log(`error:`, res.Err)
			continue
		}
		d, ok := res.Payload.(float64)
		if !ok {
			t.Error(`RESULT SHOULD BE A float64`)
			t.Fail()
		}
		if d < minDist {
			minDist = d
		}
	}
	if minDist == math.MaxFloat64 {
		t.Fail()
	}
	// t.Logf("%.6f", minDist)
}

func calcDist(input Payload) Payload {
	if input.Payload == nil {
		return Payload{Err: Error(`NO PAYLOAD`)}
	}
	p, ok := input.Payload.(pair)
	if !ok {
		return Payload{Err: Error(`PAYLOAD IS NOT A pair`)}
	}
	if p.fst == nil || p.snd == nil {
		return Payload{Err: Error(`PAIR MUST CONTAIN TWO LOCATIONS`)}
	}
	d := distance(*p.fst, *p.snd)
	return Payload{Payload: d}
}

// this is a statefull processor
type toPair struct {
	fst, snd *location
}

func (x *toPair) Process(input Payload) Payload {
	if input.Payload == nil {
		return Payload{Err: Error(`NO PAYLOAD`)}
	}
	loc, ok := input.Payload.(location)
	if !ok {
		return Payload{Err: Error(`PAYLOAD IS NOT A location`)}
	}
	x.fst, x.snd = x.snd, &loc
	return Payload{Payload: pair{x.fst, x.snd}}
}

type pair struct{ fst, snd *location }

func parse(input Payload) Payload {
	if input.Payload == nil {
		return Payload{Err: Error(`NO PAYLOAD`)}
	}
	record, ok := input.Payload.([]string)
	if !ok {
		return Payload{Err: Error(`PAYLOAD IS NOT A []string`)}
	}
	if len(record) != 3 {
		return Payload{Err: Error(`RECORD OF []string MUST HAVE 3 ITEMS`)}
	}
	lon, err := strconv.ParseFloat(record[1], 64)
	if err != nil {
		return Payload{Err: err}
	}
	lat, err := strconv.ParseFloat(record[2], 64)
	if err != nil {
		return Payload{Err: err}
	}
	return Payload{Payload: location{record[0], lon, lat}}
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
