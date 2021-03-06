// This program generates division.sql.
// It can be invoked by running `go run build.go` in current directory.

package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"runtime/debug"
	"strconv"
)

const (
	tblName       = "nested"
	provincesFile = "./data/provinces.json"
	citiesFile    = "./data/cities.json"
	areasFile     = "./data/areas.json"
	streetsFile   = "./data/streets.json"
	sqlFile       = "./division.sql"
	insertPrefix  = "INSERT INTO " + tblName + "(id, node, pid, depth, lft, rgt) VALUES("
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			log.Print(string(debug.Stack()))
		}
	}()
	loadAddress()
	trees := buildTrees()
	log.Printf("tree with %d roots", len(trees))

	assignKeys(trees)
	log.Printf("key from %d to %d", trees[0].Left, trees[len(trees)-1].Right)

	genSQLFile(trees)
}

type Area struct {
	Code       string
	Name       string
	ParentCode string
	Left       int32
	Right      int32
	SubAreas   []*Area
}

type flatNode struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	ParentCode string `json:"parent_code"`
}

var provinces, cities, areas, streets []flatNode

// load division data from files
func loadAddress() {
	// provinces
	data, err := ioutil.ReadFile(provincesFile)
	if err != nil {
		log.Fatal("ioutil.ReadFile: ", err)
	}
	err = json.Unmarshal(data, &provinces)
	if err != nil {
		log.Fatal("json.Unmarshal error: ", err)
	}
	log.Printf("got %d provinces", len(provinces))
	// log.Printf("%+v\n", provinces[:5])

	// cities
	data, err = ioutil.ReadFile(citiesFile)
	if err != nil {
		log.Fatal("ioutil.ReadFile: ", err)
	}
	err = json.Unmarshal(data, &cities)
	if err != nil {
		log.Fatal("json.Unmarshal error: ", err)
	}
	log.Printf("got %d cities", len(cities))
	// log.Printf("%+v\n", cities[:5])

	// areas
	data, err = ioutil.ReadFile(areasFile)
	if err != nil {
		log.Fatal("ioutil.ReadFile: ", err)
	}
	err = json.Unmarshal(data, &areas)
	if err != nil {
		log.Fatal("json.Unmarshal error: ", err)
	}
	log.Printf("got %d areas", len(areas))
	// log.Printf("%+v\n", areas[:5])

	// streets
	data, err = ioutil.ReadFile(streetsFile)
	if err != nil {
		log.Fatal("ioutil.ReadFile: ", err)
	}
	err = json.Unmarshal(data, &streets)
	if err != nil {
		log.Fatal("json.Unmarshal error: ", err)
	}
	log.Printf("got %d streets", len(streets))
	// log.Printf("%+v\n", streets[:5])
}

// build trees with all the division data
func buildTrees() []*Area {
	trees := make([]*Area, 0, len(provinces))

	// build provice nodes
	provinceOrder := make(map[string]int)
	for i, p := range provinces {
		trees = append(trees, &Area{
			Code:       p.Code,
			Name:       p.Name,
			ParentCode: "0",
			SubAreas:   make([]*Area, 0),
		})
		provinceOrder[p.Code] = i
	}

	// build city nodes
	cityOrder := make(map[string]int)
	for _, c := range cities {
		pCode := getProvince(c.Code)
		p := trees[provinceOrder[pCode]]

		p.SubAreas = append(p.SubAreas, &Area{
			Code:       c.Code,
			Name:       c.Name,
			ParentCode: c.ParentCode,
			SubAreas:   make([]*Area, 0),
		})
		cityOrder[c.Code] = len(p.SubAreas) - 1
	}

	// build area nodes
	areaOrder := make(map[string]int)
	for _, a := range areas {
		pCode := getProvince(a.Code)
		cCode := getCity(a.Code)
		p := trees[provinceOrder[pCode]]
		c := p.SubAreas[cityOrder[cCode]]

		c.SubAreas = append(c.SubAreas, &Area{
			Code:       a.Code,
			Name:       a.Name,
			ParentCode: a.ParentCode,
		})
		areaOrder[a.Code] = len(c.SubAreas) - 1
	}

	// build street nodes
	for _, s := range streets {
		pCode := getProvince(s.Code)
		cCode := getCity(s.Code)
		aCode := getArea(s.Code)

		p := trees[provinceOrder[pCode]]
		c := p.SubAreas[cityOrder[cCode]]
		a := c.SubAreas[areaOrder[aCode]]

		a.SubAreas = append(a.SubAreas, &Area{
			Code:       s.Code,
			Name:       s.Name,
			ParentCode: s.ParentCode,
		})
	}

	return trees
}

// number the nodes according a tree traversal
func assignKeys(trees []*Area) {
	start := int32(0)
	for _, p := range trees {
		start = indexTree(p, start)
	}
}

// generate database table initial inserting sql queries
func genSQLFile(trees []*Area) {
	f, err := os.Create(sqlFile)
	if err != nil {
		log.Panic("os.Create error: ", err)
	}
	defer f.Close()

	for _, p := range trees {
		genSQL(f, p, 1)
	}
}

func indexTree(root *Area, start int32) int32 {
	start++
	root.Left = start
	for _, sub := range root.SubAreas {
		start = indexTree(sub, start)
	}
	start++
	root.Right = start
	return start
}

func genSQL(f *os.File, area *Area, depth int32) {
	sql := bytes.NewBufferString(insertPrefix)
	sql.WriteString(area.Code)
	sql.WriteString(", '")
	sql.WriteString(area.Name)
	sql.WriteString("', ")
	sql.WriteString(area.ParentCode)
	sql.WriteString(", ")
	sql.WriteString(itoa(depth))
	sql.WriteString(", ")
	sql.WriteString(itoa(area.Left))
	sql.WriteString(", ")
	sql.WriteString(itoa(area.Right))
	sql.WriteString(");\n")

	_, err := f.Write(sql.Bytes())
	if err != nil {
		log.Panic("f.Write error: ", err, " when writting area: ", *area)
	}

	for _, sub := range area.SubAreas {
		genSQL(f, sub, depth+1)
	}
}

func getProvince(code string) string {
	p := []byte("000000")
	copy(p[:2], []byte(code)[:2])
	return string(p)
}

func getCity(code string) string {
	c := []byte("000000")
	copy(c[:4], []byte(code)[:4])
	return string(c)
}

func getArea(code string) string {
	c := []byte("000000")
	copy(c[:], []byte(code)[:6])
	return string(c)
}

func itoa(i int32) string {
	return strconv.FormatInt(int64(i), 10)
}
