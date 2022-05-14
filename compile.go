package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
)

type stringObj struct {
	b []byte
	q []byte
}

func main() {
	fmt.Println(os.Args[1])
	preCompile("test/views/index.xhtml")
}


func preCompile(file string){
	html, err := ioutil.ReadFile(file)
	if err != nil {
		fmt.Println(err)
	}

	// encode encoding
	regEncodeEncoding := regexp.MustCompile(`%!|!%`)
	html = regEncodeEncoding.ReplaceAllFunc(html, func(b []byte) []byte {
		if bytes.Equal(b, []byte("%!")) {
			return []byte("%!o!%")
		}
		return []byte("%!c!%")
	});


	stringList := []stringObj{}
	regEncodeStrings := regexp.MustCompile(`(?s)(")((?:\\[\\"]|.)*?)"|(')((?:\\[\\"]|.)*?)'|` + "(`)((?:\\\\[\\\\`]|.)*?)`" + `|<!--.*?-->|\/\*.*?\*\/|(?:\/\/|#!).*?\r?\n`)
	html = regEncodeStrings.ReplaceAllFunc(html, func(b []byte) []byte {
		data := regEncodeStrings.FindSubmatch(b)
		if bytes.Equal(data[1], []byte("\"")) {
			ind := len(stringList)
			stringList = append(stringList, stringObj{b: data[2], q: data[1]})
			return []byte("%!" + strconv.Itoa(ind) + "!%")
		}else if bytes.Equal(data[3], []byte("'")) {
			ind := len(stringList)
			stringList = append(stringList, stringObj{b: data[4], q: data[3]})
			return []byte("%!" + strconv.Itoa(ind) + "!%")
		}else if bytes.Equal(data[5], []byte("`")) {
			ind := len(stringList)
			stringList = append(stringList, stringObj{b: data[6], q: data[5]})
			return []byte("%!" + strconv.Itoa(ind) + "!%")
		}
		return []byte("")
	});


	decodeStrings := func(b []byte, num int) []byte {
		regIsNumber := regexp.MustCompile(`^-?[0-9]+(\.[0-9]+|)$`)

		regDecodeStrings := regexp.MustCompile(`%!([0-9]+)!%`)
		b = regDecodeStrings.ReplaceAllFunc(b, func(b []byte) []byte {
			data := regDecodeStrings.FindSubmatch(b)
			i, err := strconv.Atoi(string(data[1]))
			if err != nil {
				return []byte("")
			}

			if num == 1 || (num == 2 && regIsNumber.Match(stringList[i].b)) {
				return stringList[i].b
			}

			r := append(stringList[i].q, stringList[i].b...)
			r = append(r, stringList[i].q...)
			fmt.Println(string(r))
			return r
		})

		regDecodeEncoding := regexp.MustCompile(`%![oc]!%`)
		b = regDecodeEncoding.ReplaceAllFunc(b, func(b []byte) []byte {
			if bytes.Equal(b, []byte("%!o!%")) {
				return []byte("%!")
			}
			return []byte("!%")
		})

		return b
	}


	_ = decodeStrings
	// html = decodeStrings(html, 0)

}


func compress(msg string) (string, error) {
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	if _, err := gz.Write([]byte(msg)); err != nil {
		return "", err
	}
	if err := gz.Flush(); err != nil {
		return "", err
	}
	if err := gz.Close(); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b.Bytes()), nil
}

func decompress(str string) string {
	data, _ := base64.StdEncoding.DecodeString(str)
	rdata := bytes.NewReader(data)
	r,_ := gzip.NewReader(rdata)
	s, _ := ioutil.ReadAll(r)
	return string(s)
}
