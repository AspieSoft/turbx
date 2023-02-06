package compiler

import (
	"github.com/AspieSoft/go-regex/v4"
	"github.com/AspieSoft/goutil/v4"
)

func GetOpt(name []byte, opts *map[string]interface{}, escape uint8, precomp bool) (interface{}) {
	// escape: 0 = raw, 1 = raw arg, 2 = html, 3 = arg, 4 = html arg key

	//todo: handle & | operators and .obj[key] objects
	// fmt.Println(escape)

	//todo: if precomp and var is not a constant, return {{var}} as a result
	// also return {{=var}} to represent an escaped arg
	// use {{:var}} for an html arg key

	if (*opts)[string(name)] == nil {
		if escape == 0 {
			// pass with fist byte as 0 to authorize passing a var
			return regex.JoinBytes([]byte{0}, []byte("{{{"), name, []byte("}}}"))
		}else if escape == 1 {
			return regex.JoinBytes([]byte{0}, []byte("{{{="), name, []byte("}}}"))
		}else if escape == 2 {
			return regex.JoinBytes([]byte{0}, []byte("{{"), name, []byte("}}"))
		}else if escape == 3 {
			return regex.JoinBytes([]byte{0}, []byte("{{="), name, []byte("}}"))
		}else if escape == 4 {
			return regex.JoinBytes([]byte{0}, []byte("{{:"), name, []byte("}}"))
		}else{
			return nil
		}
	}

	val := goutil.CleanJSON((*opts)[string(name)])
	if escape == 0 || escape == 1 {
		return val
	}else if escape == 2 {
		return goutil.EscapeHTML(goutil.ToString[[]byte](val))
	}else if escape == 3 {
		//todo: sanitize arg from xss attacks (example: remove 'data:' from val)
		return goutil.EscapeHTMLArgs(goutil.ToString[[]byte](val))
	}else if escape == 4 {
		return regex.Comp(`[^\w_-]+`).RepStr(goutil.ToString[[]byte](val), []byte{})
	}

	return nil
}
