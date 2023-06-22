package compilerOld

import (
	"bufio"
	"bytes"
	"fmt"
	"reflect"

	"github.com/AspieSoft/go-regex/v4"
	"github.com/AspieSoft/goutil/v4"
)

var leyoutHead = regex.Compile(`\n\s+`).RepStr(bytes.TrimSpace([]byte(`
	<script src="https://instant.page/5.1.1" type="module" integrity="sha384-MWfCL6g1OTGsbSwfuMHc8+8J2u71/LA8dzlIN3ycajckxuZZmF+DNjdm7O6H3PSq"></script>
`)), []byte{'\n'})

// an optional head to add to a layout
func addLayoutHead(opts *map[string]interface{}) []byte {
	publicOpts := []byte{}
	if (*opts)["public"] != nil && reflect.TypeOf((*opts)["public"]) == goutil.VarType["map"] {
		public := (*opts)["public"].(map[string]interface{})
		
		// add public js options
		if public["js"] != nil && reflect.TypeOf(public["js"]) == goutil.VarType["map"] {
			if json, err := goutil.StringifyJSON(public["js"]); err == nil {
				publicOpts = regex.JoinBytes([]byte("<script>;const OPTS = "), json, []byte(";</script>"))
			}
		}

		// add public css options
		if public["css"] != nil && reflect.TypeOf(public["css"]) == goutil.VarType["map"] {
			publicOpts = append(publicOpts, []byte("<style>:root{")...)
			for key, val := range public["css"].(map[string]interface{}) {
				publicOpts = append(publicOpts, regex.JoinBytes([]byte("--"), key, ':', bytes.ReplaceAll(goutil.ToString[[]byte](val), []byte(";"), []byte{}), ';')...)
			}
			publicOpts = append(publicOpts, []byte("}</style>")...)
		}

	}
	return append(leyoutHead, publicOpts...)
}

func escapeChar(char byte) []byte {
	if char == '<' {
		return []byte("&lt;")
	}else if char == '>' {
		return []byte("&gt;")
	}else if char == '&' {
		return []byte("&amp;")
	}else if char == '$' {
		return []byte("&cent;")
	}

	return []byte{char}
}


// markdown funcs
func getFormInput(args *map[string][]byte) []byte {
	//todo: handle form inputs
	return nil
}

func getLinkEmbed(args *map[string][]byte) []byte {
	htmlArgs := []byte{}
	css := []byte{}
	compArgs := (*args)["args"]

	noCtrl := false

	key := []byte{}
	for i := 0; i < len(compArgs); i++ {
		if regex.Compile(`[\s\r\n]`).MatchRef(&[]byte{compArgs[i]}) {
			if bytes.Equal(key, []byte("no-controls")) {
				noCtrl = true
			}else if len(key) != 0 {
				htmlArgs = append(htmlArgs, regex.JoinBytes(' ', key)...)
			}
			key = []byte{}
			continue
		}
		
		if compArgs[i] == '=' {
			i++

			val := []byte{}
			if i < len(compArgs) && compArgs[i] == '"' || compArgs[i] == '\'' || compArgs[i] == '`' {
				q := compArgs[i]
				i++
				for i < len(compArgs) && compArgs[i] != q {
					if compArgs[i] == '\\' && i+1 < len(compArgs) {
						if regex.Compile(`[A-Za-z]`).MatchRef(&[]byte{compArgs[i]}) {
							val = append(val, compArgs[i], compArgs[i+1])
						}else{
							val = append(val, compArgs[i+1])
						}

						i += 2
					}else{
						val = append(val, compArgs[i])
						i++
					}
				}

				i++
			}else{
				for i < len(compArgs) && !regex.Compile(`[\s\r\n]`).MatchRef(&[]byte{compArgs[i]}) {
					val = append(val, compArgs[i])
				}
			}

			if bytes.Equal(key, []byte("no-controls")) {
				noCtrl = true
			}else{
				if len(val) != 0 {
					htmlArgs = append(htmlArgs, regex.JoinBytes(' ', key, '=', '"', goutil.EscapeHTMLArgs(val, '"'), '"')...)
				}else if len(key) != 0{
					htmlArgs = append(htmlArgs, regex.JoinBytes(' ', key)...)
				}
			}

			key = []byte{}
		}else if compArgs[i] == ':' {
			i++

			val := []byte{}
			for i < len(compArgs) && compArgs[i] != ';' {
				if compArgs[i] == '"' || compArgs[i] == '\'' || compArgs[i] == '`' {
					q := compArgs[i]
					val = append(val, q)
					i++
					for i < len(compArgs) && compArgs[i] != q {
						if compArgs[i] == '\\' && i+1 < len(compArgs) {
							if compArgs[i] == q || regex.Compile(`[A-Za-z]`).MatchRef(&[]byte{compArgs[i]}) {
								val = append(val, compArgs[i], compArgs[i+1])
							}else{
								val = append(val, compArgs[i+1])
							}

							i += 2
						}else{
							val = append(val, compArgs[i])
							i++
						}
					}

					val = append(val, q)
					i++
				}else{
					val = append(val, compArgs[i])
					i++
				}
			}

			css = append(css, regex.JoinBytes(key, ':', val, ';')...)
			key = []byte{}
		} else {
			key = append(key, compArgs[i])
		}
	}

	if len(key) != 0 {
		if bytes.Equal(key, []byte("no-controls")) {
			noCtrl = true
		}else{
			htmlArgs = append(htmlArgs, regex.JoinBytes(' ', key)...)
		}
	}

	if len(css) != 0 {
		htmlArgs = append(htmlArgs, regex.JoinBytes(' ', []byte("style=\""), css, '"')...)
	}

	if (*args)["emb"] == nil {
		// normal link

		//todo: prevent url from being a non http link

		return regex.JoinBytes([]byte("<a href=\""), (*args)["url"], []byte("\""), htmlArgs, '>', (*args)["name"], []byte("</a>"))
	}

	url := (*args)["url"]

	if regex.Compile(`(?i)\.(a?png|jpe?g|webp|avif|gif|jfif|pjpeg|pjp|svg|bmp|ico|cur|tiff?)$`).MatchRef(&url) {
		return regex.JoinBytes([]byte("<img src=\""), goutil.EscapeHTMLArgs(url, '"'), []byte("\" alt=\""), goutil.EscapeHTMLArgs((*args)["name"], '"'), []byte("\""), htmlArgs, []byte("/>"))
	}else if regex.Compile(`(?i)\.(mp4|mov|webm|avi|mpeg|ogv|ts|3gp2?)$`).MatchRef(&url) {
		//todo: may have videos use an optional lazy loading feature with an image until clicked

		if !noCtrl {
			htmlArgs = append(htmlArgs, []byte(" controls")...)
		}

		if bytes.ContainsRune(url, '|') {
			list := []byte{}
			for _, val := range bytes.Split(url, []byte("|")) {
				list = append(list, regex.JoinBytes([]byte("<source src=\""), goutil.EscapeHTMLArgs(val, '"'), []byte("\" type=\"video/"), regex.Compile(`^.*\.([\w_-]+)$`).RepStrComplexRef(&val, []byte("$1")), []byte("\"/>\n"))...)
			}

			return regex.JoinBytes([]byte("<video"), htmlArgs, '>', list, (*args)["name"], []byte("\n</video>"))
		}else{
			return regex.JoinBytes([]byte("<video"), htmlArgs, []byte(">\n<source src=\""), goutil.EscapeHTMLArgs(url, '"'), []byte("\" type=\"video/"), regex.Compile(`^.*\.([\w_-]+)$`).RepStrComplexRef(&url, []byte("$1")), []byte("\"/>\n"), (*args)["name"], []byte("\n</video>"))
		}
	}else if regex.Compile(`(?i)\.(mp3|wav|weba|ogg|oga|aac|midi?|opus|3gpp2?)$`).MatchRef(&url) {
		//todo: may have audio use an optional lazy loading feature with an image until clicked

		if !noCtrl {
			htmlArgs = append(htmlArgs, []byte(" controls")...)
		}

		if bytes.ContainsRune(url, '|') {
			list := []byte{}
			for _, val := range bytes.Split(url, []byte("|")) {
				list = append(list, regex.JoinBytes([]byte("<source src=\""), goutil.EscapeHTMLArgs(val, '"'), []byte("\" type=\"audio/"), regex.Compile(`^.*\.([\w_-]+)$`).RepStrComplexRef(&val, []byte("$1")), []byte("\"/>\n"))...)
			}
			return regex.JoinBytes([]byte("<audio"), htmlArgs, '>', list, (*args)["name"], []byte("\n</audio>"))
		}else{
			return regex.JoinBytes([]byte("<audio"), htmlArgs, []byte(">\n<source src=\""), goutil.EscapeHTMLArgs(url, '"'), []byte("\" type=\"audio/"), regex.Compile(`^.*\.([\w_-]+)$`).RepStrComplexRef(&url, []byte("$1")), []byte("\"/>\n"), (*args)["name"], []byte("\n</audio>"))
		}
	}else{
		//todo: may have embeds use an optional lazy loading feature with an image until clicked
		//todo: may add special case for youtube.com urls

		//todo: new idea - may have all embeds use an optional lazy loading feature
		return regex.JoinBytes([]byte("<iframe src=\""), goutil.EscapeHTMLArgs(url, '"'), []byte("\" alt=\""), (*args)["name"], '"', htmlArgs, []byte("></iframe>"))
	}
}


func hasClosingChar(char []byte, offset int, reader *bufio.Reader, b *[]byte, err *error, allowLineBreaks ...bool) bool {
	offset++
	*b, *err = reader.Peek(offset+1)

	for *err == nil && (len(allowLineBreaks) != 0 || !((*b)[offset] == '\n' || (*b)[offset] == '\r')) {
		if len(*b) >= len(char) && bytes.Equal((*b)[len(*b)-len(char):], char) {
			break
		}

		offset++
		*b, *err = reader.Peek(offset+1)
	}

	return bytes.Equal((*b)[len(*b)-len(char):], char)
}


// returning true will run the "continue" function on the pre compiler loop
func compileMarkdown(reader *bufio.Reader, write *func([]byte), b *[]byte, err *error, linePos *int, fnCont *[]fnData) bool {
	// handle http links
	if (*b)[0] == 'h' {
		*b, *err = reader.Peek(8)
		if *err == nil && regex.Compile(`^https?://`).MatchRef(b) {
			link := []byte("http")
			if (*b)[4] == 's' {
				link = append(link, 's')
			}
			link = append(link, ':', '/', '/')
			reader.Discard(len(link))
			*b, *err = reader.Peek(1)

			for *err == nil && !regex.Compile(`[\s\r\n<]|[^\w_\-\.~:/?#\[\]@!$&"'\'\(\)\*\+,;%=]`).MatchRef(b) {
				link = append(link, (*b)[0])
				reader.Discard(1)
				*b, *err = reader.Peek(1)
			}

			if len(*fnCont) != 0 && bytes.HasPrefix((*fnCont)[len(*fnCont)-1].tag, []byte("@md")) {
				(*write)(link)
				return true
			}

			(*write)(regex.JoinBytes([]byte("<a href=\""), goutil.EscapeHTMLArgs(link, '"'), []byte("\">"), link, []byte("</a>")))

			fmt.Println("debug:", string(*b))
			return true
		}
	}

	offset := 0
	if (*b)[0] == '!' {
		offset++
		*b, *err = reader.Peek(offset+1)
	}

	if (*b)[offset] == '[' && hasClosingChar([]byte{']'}, offset, reader, b, err) {
		offset = 1

		args := map[string][]byte{}
		if (*b)[0] == '!' {
			args["emb"] = []byte{1}
			offset++
		}

		*fnCont = append(*fnCont, fnData{tag: []byte("@md:link-name"), args: args, cont: []byte{}})

		reader.Discard(offset)
		*b, *err = reader.Peek(1)
		return true
	}

	if (*b)[0] == ']' && len(*fnCont) != 0 && bytes.Equal((*fnCont)[len(*fnCont)-1].tag, []byte("@md:link-name")) {
		(*fnCont)[len(*fnCont)-1].args["name"] = (*fnCont)[len(*fnCont)-1].cont
		(*fnCont)[len(*fnCont)-1].cont = []byte{}

		reader.Discard(1)
		*b, *err = reader.Peek(1)

		if (*b)[0] == '(' && hasClosingChar([]byte{')'}, offset, reader, b, err) {
			(*fnCont)[len(*fnCont)-1].tag = []byte("@md:link-url")
			reader.Discard(1)
			*b, *err = reader.Peek(1)
		}else if (*b)[0] == '{' && hasClosingChar([]byte{'}'}, offset, reader, b, err, true) {
			(*fnCont)[len(*fnCont)-1].tag = []byte("@md:link-args")
			(*fnCont)[len(*fnCont)-1].args["form"] = []byte{1}
			reader.Discard(1)
			*b, *err = reader.Peek(1)
		}else{
			if res := getFormInput(&(*fnCont)[len(*fnCont)-1].args); res != nil {
				*fnCont = (*fnCont)[:len(*fnCont)-1]
				(*write)(res)
			}else{
				res := []byte{}
				if (*fnCont)[len(*fnCont)-1].args["emb"] != nil {
					res = []byte{'!'}
				}
				res = append(res, regex.JoinBytes('[', (*fnCont)[len(*fnCont)-1].args["name"], ']')...)
				*fnCont = (*fnCont)[:len(*fnCont)-1]
				(*write)(res)
			}
		}

		return true
	}

	if (*b)[0] == ')' && len(*fnCont) != 0 && bytes.Equal((*fnCont)[len(*fnCont)-1].tag, []byte("@md:link-url")) {
		(*fnCont)[len(*fnCont)-1].args["url"] = (*fnCont)[len(*fnCont)-1].cont
		(*fnCont)[len(*fnCont)-1].cont = []byte{}

		reader.Discard(1)
		*b, *err = reader.Peek(1)

		if (*b)[0] == '{' && hasClosingChar([]byte{'}'}, offset, reader, b, err, true) {
			(*fnCont)[len(*fnCont)-1].tag = []byte("@md:link-args")
			reader.Discard(1)
			*b, *err = reader.Peek(1)
		}else{
			if res := getLinkEmbed(&(*fnCont)[len(*fnCont)-1].args); res != nil {
				*fnCont = (*fnCont)[:len(*fnCont)-1]
				(*write)(res)
			}else{
				res := []byte{}
				if (*fnCont)[len(*fnCont)-1].args["emb"] != nil {
					res = []byte{'!'}
				}
				res = append(res, regex.JoinBytes('[', (*fnCont)[len(*fnCont)-1].args["name"], ']', '(', (*fnCont)[len(*fnCont)-1].args["url"], ')')...)
				*fnCont = (*fnCont)[:len(*fnCont)-1]
				(*write)(res)
			}
		}

		return true
	}

	if (*b)[0] == '}' && len(*fnCont) != 0 && bytes.Equal((*fnCont)[len(*fnCont)-1].tag, []byte("@md:link-args")) {
		(*fnCont)[len(*fnCont)-1].args["args"] = (*fnCont)[len(*fnCont)-1].cont
		(*fnCont)[len(*fnCont)-1].cont = []byte{}

		reader.Discard(1)
		*b, *err = reader.Peek(1)

		if (*fnCont)[len(*fnCont)-1].args["form"] != nil {
			if res := getFormInput(&(*fnCont)[len(*fnCont)-1].args); res != nil {
				*fnCont = (*fnCont)[:len(*fnCont)-1]
				(*write)(res)
				return true
			}
		}else if res := getLinkEmbed(&(*fnCont)[len(*fnCont)-1].args); res != nil{
			*fnCont = (*fnCont)[:len(*fnCont)-1]
			(*write)(res)
			return true
		}

		res := []byte{}
		if (*fnCont)[len(*fnCont)-1].args["emb"] != nil {
			res = []byte{'!'}
		}
		res = append(res, regex.JoinBytes('[', (*fnCont)[len(*fnCont)-1].args["name"], ']', '(', (*fnCont)[len(*fnCont)-1].args["url"], ')', '{', (*fnCont)[len(*fnCont)-1].args["args"], '}')...)
		*fnCont = (*fnCont)[:len(*fnCont)-1]
		(*write)(res)

		return true
	}

	// handle bold and italic text
	if (*b)[0] == '*' {
		*b, *err = reader.Peek(4)
		if *err == nil {
			size := 1
			if (*b)[1] == '*' {
				size++
				if (*b)[2] == '*' {
					size++
				}
			}
	
			if (*b)[size] != '*' {
				offset := size
				*b, *err = reader.Peek(offset+size)
				
				text := []byte{}
				for *err == nil && !(bytes.Equal((*b)[offset:], bytes.Repeat([]byte{'*'}, size)) || (*b)[offset] == '\n' || (*b)[offset] == '\r') {
					text = append(text, (*b)[offset])
	
					offset++
					*b, *err = reader.Peek(offset+size)
				}
	
				*b, *err = reader.Peek(offset+size)
				if bytes.Equal((*b)[offset:], bytes.Repeat([]byte{'*'}, size)) {
					if size == 1 {
						(*write)(regex.JoinBytes([]byte("<em>"), text, []byte("</em>")))
					}else if size == 2 {
						(*write)(regex.JoinBytes([]byte("<strong>"), text, []byte("</strong>")))
					}else if size == 3 {
						(*write)(regex.JoinBytes([]byte("<em><strong>"), text, []byte("</strong></em>")))
					}
	
					reader.Discard(offset+size)
					*b, *err = reader.Peek(1)
					return true
				}
			}
		}
	}

	// handle underline text
	if (*b)[0] == '_' {
		*b, *err = reader.Peek(3)
		if *err == nil && (*b)[1] == '_' && (*b)[2] != '_' {
			offset := 2
			*b, *err = reader.Peek(offset+2)

			text := []byte{}
			for *err == nil && !(bytes.Equal((*b)[offset:], []byte("__")) || (*b)[offset] == '\n' || (*b)[offset] == '\r') {
				text = append(text, (*b)[offset])

				offset++
				*b, *err = reader.Peek(offset+2)
			}

			*b, *err = reader.Peek(offset+2)
			if bytes.Equal((*b)[offset:], []byte("__")) {
				(*write)(regex.JoinBytes([]byte("<span style=\"text-decoration: underline;\">"), text, []byte("</span>")))

				reader.Discard(offset+2)
				*b, *err = reader.Peek(1)
				return true
			}
		}
	}

	// handle strikeout text
	if (*b)[0] == '~' {
		*b, *err = reader.Peek(3)
		if *err == nil && (*b)[1] == '~' && (*b)[2] != '~' {
			offset := 2
			*b, *err = reader.Peek(offset+2)

			text := []byte{}
			for *err == nil && !(bytes.Equal((*b)[offset:], []byte("~~")) || (*b)[offset] == '\n' || (*b)[offset] == '\r') {
				text = append(text, (*b)[offset])

				offset++
				*b, *err = reader.Peek(offset+2)
			}

			*b, *err = reader.Peek(offset+2)
			if bytes.Equal((*b)[offset:], []byte("~~")) {
				(*write)(regex.JoinBytes([]byte("<del>"), text, []byte("</del>")))

				reader.Discard(offset+2)


				// check for insert text
				offset := 0
				*b, *err = reader.Peek(offset+2)

				text := []byte{}
				for *err == nil && !(bytes.Equal((*b)[offset:], []byte("~~")) || (*b)[offset] == '\n' || (*b)[offset] == '\r') {
					text = append(text, (*b)[offset])

					offset++
					*b, *err = reader.Peek(offset+2)
				}

				*b, *err = reader.Peek(offset+2)
				if bytes.Equal((*b)[offset:], []byte("~~")) {
					if text[0] == ' ' {
						(*write)([]byte{' '})
						text = text[1:]
					}
					(*write)(regex.JoinBytes([]byte("<ins>"), text, []byte("</ins>")))

					reader.Discard(offset+2)
				}


				*b, *err = reader.Peek(1)
				return true
			}
		}
	}

	// handle headers
	if *linePos == 0 && (*b)[0] == '#' {
		*b, *err = reader.Peek(7)
		if *err == nil {
			size := 1
			for size < 7 && (*b)[size] == '#' {
				size++
			}

			if size < 7 {
				offset := size
				*b, *err = reader.Peek(offset+size)
				
				text := []byte{}
				for *err == nil && !((*b)[offset] == '\n' || (*b)[offset] == '\r') {
					if len(text) != 0 || !(len(text) == 0 && regex.Compile(`[\s\r\n]`).MatchRef(&[]byte{(*b)[offset]})) {
						text = append(text, (*b)[offset])
					}

					offset++
					*b, *err = reader.Peek(offset+1)
				}

				(*write)(regex.JoinBytes([]byte("<h"), size, '>', text, []byte("</h"), size, '>', '\n'))
				
				reader.Discard(offset+1)
				*b, *err = reader.Peek(1)
				return true
			}
		}
	}

	if *linePos == 0 && (*b)[0] == '-' {
		*b, *err = reader.Peek(3)
		if (*b)[1] == '-' && (*b)[2] == '-' {
			offset := 3
			*b, *err = reader.Peek(offset+1)
			for *err == nil && ((*b)[offset] == '-' || (*b)[offset] == ' ') {
				offset++
				*b, *err = reader.Peek(offset+1)
			}

			if (*b)[offset] == '\n' || (*b)[offset] == '\r' {
				reader.Discard(offset+1)
				*b, *err = reader.Peek(1)
				(*write)([]byte("<hr/>"))
				return true
			}
		}
	}

	//todo: compile markdown while reading file

	return false
}
