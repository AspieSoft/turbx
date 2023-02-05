package compiler

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/AspieSoft/go-liveread"
	"github.com/AspieSoft/go-regex/v4"
	"github.com/AspieSoft/goutil/v4"
)

type Config struct {
	Root string
	Ext string
	Static string
	StaticUrl string
}

var compilerConfig Config

func SetConfig(config Config) error {
	if config.Root != "" {
		path, err := filepath.Abs(config.Root)
		if err != nil {
			return err
		}
		compilerConfig.Root = path
	}

	if config.Static != "" {
		path, err := filepath.Abs(config.Root)
		if err != nil {
			return err
		}
		compilerConfig.Static = path
	}

	if config.Ext != "" {
		if strings.HasPrefix(config.Ext, ".") {
			config.Ext = config.Ext[1:]
		}
		compilerConfig.Ext = config.Ext
	}

	if config.StaticUrl != "" {
		if strings.HasSuffix(config.StaticUrl, "/") {
			config.StaticUrl = config.StaticUrl[:len(config.StaticUrl)-1]
		}
		compilerConfig.StaticUrl = config.StaticUrl
	}

	return nil
}

func init(){
	root, err := filepath.Abs("views")
	if err != nil {
		root = "views"
	}

	static, err := filepath.Abs("public")
	if err != nil {
		root = "public"
	}

	compilerConfig = Config{
		Root: root,
		Ext: "html",
		Static: static,
		StaticUrl: "",
	}
}

func Close(){
	// time.Sleep(3 * time.Second)
}

type htmlArgs struct {
	args map[string][]byte
	ind []string
	tag []byte
	close uint8
}

func PreCompile(path string, opts map[string]interface{}) error {
	path, err := goutil.JoinPath(compilerConfig.Root, path + "." + compilerConfig.Ext)
	if err != nil {
		return err
	}

	html := []byte{}
	done := uint8(0)
	preCompile(path, &opts, &htmlArgs{}, &html, &done)

	return nil
}


func preCompile(path string, options *map[string]interface{}, arguments *htmlArgs, html *[]byte, done *uint8){
	reader, err := liveread.Read(path)
	if err != nil {
		*html = []byte(err.Error())
		*done = 2
		return
	}

	htmlTags := [][]byte{}
	htmlTagsInd := uint(0)

	var buf byte
	for err == nil {
		buf, err = reader.PeekByte(0)
		if buf == 0 {
			break
		}

		if buf == '<' {
			args := htmlArgs{
				args: map[string][]byte{},
				ind: []string{},
			}
			argInd := 0

			ind := uint(1)
			b, e := reader.PeekByte(ind)
			if b == '/' {
				args.close = 1
				ind++

				b, e = reader.PeekByte(ind)
			}

			if regex.Comp(`[\w_]`).MatchRef(&[]byte{b}) {
				args.tag = []byte{b}
				ind++

				for e == nil {
					b, e = reader.PeekByte(ind)
					ind++
					if b == 0 {
						break
					}

					if b == '/' {
						if b2, e2 := reader.PeekByte(ind); e2 == nil && b2 == '>' {
							ind++
							args.close = 2
							break
						}
					}else if b == '>' {
						if args.close == 0 {
							args.close = 3
						}
						break
					}else if regex.Comp(`[\s\r\n]`).MatchRef(&[]byte{b}) {
						break
					}

					args.tag = append(args.tag, b)
				}

				_ = htmlTags
				_ = htmlTagsInd

				for e == nil && args.close == 0 {
					b, e = reader.PeekByte(ind)
					ind++
					if b == 0 {
						break
					}

					if b == '/' {
						if b2, e2 := reader.PeekByte(ind); e2 == nil && b2 == '>' {
							ind++
							args.close = 2
							break
						}
					}else if b == '>' {
						if args.close == 0 {
							args.close = 3
						}
						break
					}else if regex.Comp(`[\s\r\n]`).MatchRef(&[]byte{b}) {
						continue
					}
					
					var q byte
					if b == '"' || b == '\'' || b == '`' {
						q = b
						b, e = reader.PeekByte(ind)
						ind++
					}

					key := []byte{}
					for e == nil && ((q == 0 && regex.Comp(`[^\s\r\n=/>]`).MatchRef(&[]byte{b})) || (q != 0 && b != q)) {
						if q != 0 && b == '\\' {
							b, e = reader.PeekByte(ind)
							ind++
							if b != q && b != '\\' {
								key = append(key, '\\')
							}
						}
						
						key = append(key, b)
						b, e = reader.PeekByte(ind)
						ind++
					}

					if b == '>' || b == '/' {
						ind--
					}

					if b != '=' {
						//todo: handle {{key}} var tags
						isVar := uint8(0)
						if bytes.HasPrefix(key, []byte("{{")) && bytes.HasSuffix(key, []byte("}}")) {
							key = key[2:len(key)-2]
							isVar++

							if bytes.HasPrefix(key, []byte("{")) && bytes.HasSuffix(key, []byte("}")) {
								key = key[1:len(key)-1]
								isVar++
							}else if bytes.HasPrefix(key, []byte("{")) {
								key = key[1:]
							}else if bytes.HasSuffix(key, []byte("}")) {
								key = key[:len(key)-1]
							}
						}

						i := strconv.Itoa(argInd)
						args.args[i] = append([]byte{isVar}, key...)
						args.ind = append(args.ind, i)
						argInd++
						continue
					}

					b, e = reader.PeekByte(ind)
					ind++

					q = 0
					if b == '"' || b == '\'' || b == '`' {
						q = b
						b, e = reader.PeekByte(ind)
						ind++
					}

					val := []byte{}
					for e == nil && ((q == 0 && regex.Comp(`[^\s\r\n=/>]`).MatchRef(&[]byte{b})) || (q != 0 && b != q)) {
						if q != 0 && b == '\\' {
							b, e = reader.PeekByte(ind)
							ind++
							if b != q && b != '\\' {
								val = append(val, '\\')
							}
						}
						
						val = append(val, b)
						b, e = reader.PeekByte(ind)
						ind++
					}

					if b == '>' || b == '/' {
						ind--
					}

					isVar := uint8(0)
					if len(key) >= 2 && key[0] == '{' && key[1] == '{' {
						key = key[2:]
						isVar++

						if len(key) >= 1 && key[0] == '{' {
							key = key[1:]
							isVar++
						}

						if b2, e2 := reader.Get(ind, 3); e2 == nil && b2[0] == '}' && b2[1] == '}' {
							ind += 2
							if b2[2] == '}' {
								ind++
							}else{
								isVar = 1
							}
						}else if len(val) >= 2 && val[len(val)-2] == '}' && val[len(val)-1] == '}' {
							val = val[:len(val)-2]
							if len(val) >= 1 && val[len(val)-1] == '}' {
								val = val[:len(val)-1]
							}else{
								isVar = 1
							}
						}else if isVar == 2 {
							key = append([]byte("{{{"), key...)
							isVar = 0
						} else {
							key = append([]byte("{{"), key...)
							isVar = 0
						}
					}

					k := string(regex.Comp(`^([\w_-]+).*$`).RepStrCompRef(&key, []byte("$1")))
					if k == "" {
						k = string(regex.Comp(`^([\w_-]+).*$`).RepStrCompRef(&val, []byte("$1")))
					}

					if args.args[k] != nil {
						i := 1
						for args.args[k+":"+strconv.Itoa(i)] != nil {
							i++
						}
						args.args[k+":"+strconv.Itoa(i)] = append([]byte{isVar}, val...)
						args.ind = append(args.ind, k+":"+strconv.Itoa(i))
					}else{
						args.args[k] = append([]byte{isVar}, val...)
						args.ind = append(args.ind, k)
					}
				}

				fmt.Println(args.args)
	
				/* runNext := true
				if b == '>' {
					if args["@CLOSE"] == nil {
						args["@CLOSE"] = []byte{0}
					}

					htmlTags = append(htmlTags, []byte{})
					handleHtmlTag(&htmlTags[htmlTagsInd], options, &args)
					htmlTagsInd++
					*html = append(*html, 0)

					reader.Discard(ind)
					continue
				}else if b == '/' {
					if c, ce := reader.PeekByte(ind); ce == nil && c == '>' {
						args["@CLOSE"] = []byte{2}
						
						htmlTags = append(htmlTags, []byte{})
						handleHtmlTag(&htmlTags[htmlTagsInd], options, &args)
						htmlTagsInd++
						*html = append(*html, 0)

						reader.Discard(ind+1)
						continue
					}else{
						runNext = false
					}
				}

				if runNext {
					for e == nil {
						b, e = reader.PeekByte(ind)
						ind++
						if b == 0 {
							break
						}

						if b == '>' {
							if args["@CLOSE"] == nil {
								args["@CLOSE"] = []byte{0}
							}
							break
						}else if b == '/' {
							if c, ce := reader.PeekByte(ind); ce == nil && c == '>' {
								args["@CLOSE"] = []byte{2}
								break
							}
						}
					}
				} */

			}

		}

		*html = append(*html, buf)
		reader.Discard(1)
	}
}

func handleHtmlTag(html *[]byte, options *map[string]interface{}, arguments *map[string][]byte){
	//todo: handle html tag
	fmt.Println(arguments)
}
