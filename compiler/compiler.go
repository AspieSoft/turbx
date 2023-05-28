package compiler

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/AspieSoft/go-liveread"
	"github.com/AspieSoft/go-regex/v4"
	"github.com/AspieSoft/goutil/v5"
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


type tagData struct {
	tag []byte
	attr []byte
}

// list of self naturally closing html tags
var singleHtmlTags [][]byte = [][]byte{
	[]byte("br"),
	[]byte("hr"),
	[]byte("wbr"),
	[]byte("meta"),
	[]byte("link"),
	[]byte("param"),
	[]byte("base"),
	[]byte("input"),
	[]byte("img"),
	[]byte("area"),
	[]byte("col"),
	[]byte("command"),
	[]byte("embed"),
	[]byte("keygen"),
	[]byte("source"),
	[]byte("track"),
}

// @tag: tag to detect
// @attr: required attr to consider
var emptyContentTags []tagData = []tagData{
	{[]byte("script"), []byte("src")},
	{[]byte("iframe"), nil},
}

type htmlArgs struct {
	args map[string][]byte
	ind []string
	tag []byte
	close uint8
}

type handleHtmlData struct {
	html *[]byte
	options *map[string]interface{}
	arguments *htmlArgs
	compileError *error

	stopChan bool
}

func PreCompile(path string, opts map[string]interface{}) error {
	path, err := goutil.FS.JoinPath(compilerConfig.Root, path + "." + compilerConfig.Ext)
	if err != nil {
		return err
	}

	tagChan := make(chan handleHtmlData)
	compChan := make(chan handleHtmlData)

	go func(){
		for {
			handleHtml := <-tagChan
			if handleHtml.stopChan {
				break
			}
			
			handleHtmlTag(handleHtml.html, handleHtml.options, handleHtml.arguments, handleHtml.compileError)
		}
	}()

	go func(){
		for {
			handleHtml := <-compChan
			if handleHtml.stopChan {
				break
			}
			
			handleHtmlComponent(handleHtml.html, handleHtml.options, handleHtml.arguments, handleHtml.compileError)
		}
	}()

	html := []byte{0}
	preCompile(path, &opts, &htmlArgs{}, &html, &err, tagChan, compChan)

	fmt.Println("----------\n", string(html[1:]))

	return nil
}


func preCompile(path string, options *map[string]interface{}, arguments *htmlArgs, html *[]byte, compileError *error, tagChan chan handleHtmlData, compChan chan handleHtmlData){
	reader, err := liveread.Read(path)
	if err != nil {
		*compileError = err
		(*html)[0] = 2
		return
	}

	htmlRes := []byte{}
	htmlTags := []*[]byte{}
	htmlTagsErr := []*error{}

	var buf byte
	for err == nil {
		buf, err = reader.PeekByte(0)
		if buf == 0 {
			break
		}

		// handle html tags
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
				
				// get tag
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

				if len(args.tag) > 0 {
					// get args
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

					// handle html tags
					if e == nil && args.close != 0 {
						reader.Discard(ind)

						// args.close:
						// 0 = failed to close (<tag)
						// 1 = </tag>
						// 2 = <tag/> (</tag/>)
						// 3 = <tag>

						if args.tag[0] == '_' {
							//todo: handle function tags (<_myFunc>)

							if args.close == 3 {
								//todo: get content

							}

							//todo: handle "if" and "each" functions in sync, instead of using concurrent goroutines
							// may think about using a concurrent channel for other functions

						}else if args.tag[0] == bytes.ToUpper([]byte{args.tag[0]})[0] {
							//todo: handle component tags (<MyComponent>)

							fmt.Println(args)

							if args.close == 3 {
								//todo: get content

							}

							//todo: handle components with a channel in place of a goroutine (like a queue) (just like how normal tags are handled)
							// compChan <- handleHtmlData{&htmlCont, options, &args, &compErr, false}
							
						}else{
							// handle normal tags
							if args.close == 3 && goutil.Contains(singleHtmlTags, bytes.ToLower(args.tag)) {
								args.close = 2
							}

							htmlCont := []byte{0}
							var compErr error
							htmlTags = append(htmlTags, &htmlCont)
							htmlTagsErr = append(htmlTagsErr, &compErr)

							// pass through channel instead of a goroutine (like a queue)
							// go handleHtmlTag(&htmlCont, options, &args, &compErr)
							tagChan <- handleHtmlData{&htmlCont, options, &args, &compErr, false}

							htmlRes = append(htmlRes, 0)
						}

						continue
					}
				}
			}

		}

		htmlRes = append(htmlRes, buf)
		reader.Discard(1)
	}

	// stop concurrent channels from running
	tagChan <- handleHtmlData{stopChan: true}
	compChan <- handleHtmlData{stopChan: true}

	// merge html tags when done
	htmlTagsInd := uint(0)
	i := bytes.IndexByte(htmlRes, 0)
	for i != -1 {
		*html = append(*html, htmlRes[:i]...)
		htmlRes = htmlRes[i+1:]

		htmlCont := htmlTags[htmlTagsInd]
		for (*htmlCont)[0] == 0 {
			time.Sleep(10 * time.Nanosecond)
		}

		if (*htmlCont)[0] == 2 {
			*compileError = *htmlTagsErr[htmlTagsInd]
			(*html)[0] = 2
			return
		}

		*html = append(*html, (*htmlCont)[1:]...)
		htmlTagsInd++

		i = bytes.IndexByte(htmlRes, 0)
	}

	*html = append(*html, htmlRes...)
	(*html)[0] = 1
}

func handleHtmlTag(html *[]byte, options *map[string]interface{}, arguments *htmlArgs, compileError *error){
	if arguments.close == 1 {
		(*html) = append((*html), regex.JoinBytes([]byte{'<', '/'}, arguments.tag, '>')...)
		(*html)[0] = 1
		return
	}

	sort.Strings(arguments.ind)

	for _, v := range arguments.ind {
		if arguments.args[v][0] == 0 {
			arguments.args[v] = arguments.args[v][1:]
		}else if arguments.args[v][0] == 1 {
			esc := uint8(3)
			if _, err := strconv.Atoi(v); err == nil {
				esc = 4
			}

			arg := GetOpt(arguments.args[v][1:], options, esc, true, true)
			if goutil.IsZeroOfUnderlyingType(arg) {
				delete(arguments.args, v)
				continue
			}else{
				arguments.args[v] = goutil.Conv.ToBytes(arg)
			}
		}else if arguments.args[v][0] == 2 {
			arg := GetOpt(arguments.args[v][1:], options, 1, true, true)
			if goutil.IsZeroOfUnderlyingType(arg) {
				delete(arguments.args, v)
				continue
			}else{
				arguments.args[v] = goutil.Conv.ToBytes(arg)
			}
		}

		if regex.Comp(`:([0-9]+)$`).Match([]byte(v)) {
			k := string(regex.Comp(`:([0-9]+)$`).RepStr([]byte(v), []byte{}))
			if arguments.args[k] == nil {
				arguments.args[k] = []byte{}
			}
			arguments.args[k] = append(append(arguments.args[k], ' '), arguments.args[v]...)
			delete(arguments.args, v)
		}
	}

	args := [][]byte{}
	for _, v := range arguments.ind {
		if arguments.args[v] != nil && len(arguments.args[v]) != 0 {
			if _, err := strconv.Atoi(v); err == nil {
				args = append(args, arguments.args[v])
			}else{
				if bytes.HasPrefix(arguments.args[v], []byte{0, '{', '{'}) && bytes.HasSuffix(arguments.args[v], []byte("}}")) {
					arguments.args[v] = arguments.args[v][1:]

					size := 2
					if arguments.args[v][2] == '{' && arguments.args[v][len(arguments.args[v])-3] == '}' {
						size = 3
					}

					if arguments.args[v][size] == '=' {
						args = append(args, regex.JoinBytes(bytes.Repeat([]byte("{"), size), v, arguments.args[v][size:len(arguments.args[v])-size], bytes.Repeat([]byte("}"), size)))
					}
				}else{
					arguments.args[v] = regex.Comp(`({{+|}}+)`).RepFunc(arguments.args[v], func(data func(int) []byte) []byte {
						return bytes.Join(bytes.Split(data(1), []byte{}), []byte{'\\'})
					})

					//todo: check local js and css link args for .min files

					args = append(args, regex.JoinBytes(v, []byte{'=', '"'}, goutil.HTML.EscapeArgs(arguments.args[v], '"'), '"'))
				}

			}
		}
	}

	sort.Slice(args, func(i, j int) bool {
		a := bytes.Split(args[i], []byte{'='})[0]
		b := bytes.Split(args[j], []byte{'='})[0]

		if a[0] == 0 {
			a = a[1:]
		}
		if b[0] == 0 {
			b = b[1:]
		}

		a = bytes.Trim(a, "{}")
		b = bytes.Trim(b, "{}")
		
		if a[0] != ':' && b[0] == ':' {
			return true
		}

		return bytes.Compare(a, b) == -1
	})

	if len(args) == 0 {
		(*html) = append((*html), regex.JoinBytes('<', arguments.tag)...)
	}else{
		(*html) = append((*html), regex.JoinBytes('<', arguments.tag, ' ', bytes.Join(args, []byte{' '}))...)
	}

	if arguments.close == 2 {
		(*html) = append((*html), '/', '>')
	}else{
		(*html) = append((*html), '>')
	}

	(*html)[0] = 1
}

func handleHtmlFunc(html *[]byte, options *map[string]interface{}, arguments *htmlArgs, compileError *error){
	//todo: handle function html tag
	// fmt.Println(arguments)

	// set first index to 1 to mark as ready
	// set to 2 for an error
	*compileError = errors.New("this method has not been setup yet")
	(*html)[0] = 2
}

func handleHtmlComponent(html *[]byte, options *map[string]interface{}, arguments *htmlArgs, compileError *error){
	// note: components cannot wait in the same channel without possibly getting stuck (ie: waiting for a parent that is also waiting for itself)

	//todo: handle component html tag
	// fmt.Println(arguments)

	// set first index to 1 to mark as ready
	// set to 2 for an error
	*compileError = errors.New("this method has not been setup yet")
	(*html)[0] = 2
}
