package compiler

import (
	"bufio"
	"bytes"
	"reflect"

	"github.com/AspieSoft/go-regex/v4"
	"github.com/AspieSoft/goutil/v3"
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
				publicOpts = append(publicOpts, regex.JoinBytes([]byte("--"), key, ':', bytes.ReplaceAll(goutil.ToByteArray(val), []byte(";"), []byte{}), ';')...)
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

// returning true will run the "continue" function on the pre compiler loop
func compileMarkdown(reader *bufio.Reader, write *func([]byte), b *[]byte, err *error, linePos *int) bool {
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

			(*write)(regex.JoinBytes([]byte("<a href=\""), goutil.EscapeHTMLArgs(link), []byte("\">"), link, []byte("</a>")))
			return true
		}
	}

	//todo: handle md links
	if (*b)[0] == '[' {
		//todo: may need to use similar method to components to allow markdown within inner text

		offset := 1
		*b, *err = reader.Peek(offset+1)

		text := []byte{}
		for *err == nil && !((*b)[offset] == ']' || (*b)[offset] == '\n' || (*b)[offset] == '\r') {
			text = append(text, (*b)[offset])

			offset++
			*b, *err = reader.Peek(offset+1)
		}

		if (*b)[offset] == ']' {
			offset++
			*b, *err = reader.Peek(offset+1)

			if (*b)[offset] == '(' {
				offset++
				*b, *err = reader.Peek(offset+1)

				url := []byte{}
				for *err == nil && !((*b)[offset] == ')' || (*b)[offset] == '\n' || (*b)[offset] == '\r') {
					url = append(url, (*b)[offset])
		
					offset++
					*b, *err = reader.Peek(offset+1)
				}

				if (*b)[offset] == ')' {
					//todo: may be able to use this to verify the link, then run another method to compile other markdown within the link text

					(*write)(regex.JoinBytes([]byte("<a href=\""), goutil.EscapeHTMLArgs(url), '"', '>', text, []byte("</a>")))

					reader.Discard(offset+1)
					*b, *err = reader.Peek(1)
					return true
				}
			}
		}
	}

	// handle md embeds
	if (*b)[0] == '!' {
		*b, *err = reader.Peek(2)
		if (*b)[1] == '[' {
			offset := 2
			*b, *err = reader.Peek(offset+1)

			text := []byte{}
			for *err == nil && !((*b)[offset] == ']' || (*b)[offset] == '\n' || (*b)[offset] == '\r') {
				text = append(text, (*b)[offset])

				offset++
				*b, *err = reader.Peek(offset+1)
			}

			if (*b)[offset] == ']' {
				offset++
				*b, *err = reader.Peek(offset+1)

				if (*b)[offset] == '(' {
					offset++
					*b, *err = reader.Peek(offset+1)

					url := []byte{}
					for *err == nil && !((*b)[offset] == ')' || (*b)[offset] == '\n' || (*b)[offset] == '\r') {
						url = append(url, (*b)[offset])

						offset++
						*b, *err = reader.Peek(offset+1)
					}

					if (*b)[offset] == ')' {
						//todo: may use this to verify the link, then run another method to compile other markdown within the link text (similar to the normal link handler)

						//todo: optionally grab additional html args from '{attrs}' right after content (also merge css styles)
						// may add a seperate function to handle this

						//todo: handle embeding 'url' with alt 'text'

						if regex.Compile(`(?i)\.(a?png|jpe?g|webp|avif|gif|jfif|pjpeg|pjp|svg|bmp|ico|cur|tiff?)$`).MatchRef(&url) {
							(*write)(regex.JoinBytes([]byte("<img src=\""), goutil.EscapeHTMLArgs(url), []byte("\" alt=\""), goutil.EscapeHTMLArgs(text), []byte("\"/>")))
						}else if regex.Compile(`(?i)\.(mp4|mov|webm|avi|mpeg|ogv|ts|3gp2?)$`).MatchRef(&url) {
							//todo: may have videos use an optional lazy loading feature with an image until clicked

							if bytes.ContainsRune(url, '|') {
								list := []byte{}
								for _, val := range bytes.Split(url, []byte("|")) {
									list = append(list, regex.JoinBytes([]byte("<source src=\""), goutil.EscapeHTMLArgs(val), []byte("\" type=\"video/"), regex.Compile(`^.*\.([\w_-]+)$`).RepStrComplexRef(&val, []byte("$1")), []byte("\"/>\n"))...)
								}
								(*write)(regex.JoinBytes([]byte("<video controls>"), list, text, []byte("\n</video>")))
							}else{
								(*write)(regex.JoinBytes([]byte("<video controls>\n<source src=\""), goutil.EscapeHTMLArgs(url), []byte("\" type=\"video/"), regex.Compile(`^.*\.([\w_-]+)$`).RepStrComplexRef(&url, []byte("$1")), []byte("\"/>\n"), text, []byte("\n</video>")))
							}
						}else if regex.Compile(`(?i)\.(mp3|wav|weba|ogg|oga|aac|midi?|opus|3gpp2?)$`).MatchRef(&url) {
							//todo: may have audio use an optional lazy loading feature with an image until clicked

							if bytes.ContainsRune(url, '|') {
								list := []byte{}
								for _, val := range bytes.Split(url, []byte("|")) {
									list = append(list, regex.JoinBytes([]byte("<source src=\""), goutil.EscapeHTMLArgs(val), []byte("\" type=\"audio/"), regex.Compile(`^.*\.([\w_-]+)$`).RepStrComplexRef(&val, []byte("$1")), []byte("\"/>\n"))...)
								}
								(*write)(regex.JoinBytes([]byte("<audio controls>"), list, text, []byte("\n</audio>")))
							}else{
								(*write)(regex.JoinBytes([]byte("<audio controls>\n<source src=\""), goutil.EscapeHTMLArgs(url), []byte("\" type=\"audio/"), regex.Compile(`^.*\.([\w_-]+)$`).RepStrComplexRef(&url, []byte("$1")), []byte("\"/>\n"), text, []byte("\n</audio>")))
							}
						}else{
							//todo: may have embeds use an optional lazy loading feature with an image until clicked
							//todo: may add special case for youtube.com urls
							(*write)(regex.JoinBytes([]byte("<iframe src=\""), goutil.EscapeHTMLArgs(url), []byte("\" alt=\""), text, []byte("\"></iframe>")))
						}


						reader.Discard(offset+1)
						*b, *err = reader.Peek(1)
						return true
					}
				}
			}
		}
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

	//todo: compile markdown while reading file

	return false
}
