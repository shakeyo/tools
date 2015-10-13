package main

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"text/template"
	"unicode"
)

const (
	SYMBOL = iota
	STRUCT_BEGIN
	STRUCT_END
	DATA_TYPE
	ARRAY_TYPE
	TK_EOF
)

var (
	datatypes = map[string]bool{
		"integer": true,
		"string":  true,
		"bytes":   true,
		"byte":    true,
		"boolean": true,
		"float":   true,
	}

	funcs map[string]lang_type
)

var (
	TOKEN_EOF = &token{typ: TK_EOF}
)

type func_info struct {
	T string `json:"t"` // type
	R string `json:"r"` // read
	W string `json:"w"` // write
}

type lang_type struct {
	Go func_info `json:"go"` // golang
	Cs func_info `json:"cs"` // c#
}
type (
	field_info struct {
		Name  string
		Typ   string
		Array bool
	}
	struct_info struct {
		Name   string
		Fields []field_info
	}
)

type token struct {
	typ     int
	literal string
	r       rune
}

func syntax_error(p *Parser) {
	log.Fatal("syntax error @line:", p.lexer.lineno)
}

type Lexer struct {
	reader *bytes.Buffer
	lineno int
}

func (lex *Lexer) init(r io.Reader) {
	bts, err := ioutil.ReadAll(r)
	if err != nil {
		log.Println(err)
	}

	// 清除注释
	re := regexp.MustCompile("(?m:^#(.*)$)")
	bts = re.ReplaceAllLiteral(bts, nil)
	lex.reader = bytes.NewBuffer(bts)
	lex.lineno = 1
}

func (lex *Lexer) next() (t *token) {
	defer func() {
		//log.Println(t)
	}()
	var r rune
	var err error
	for {
		r, _, err = lex.reader.ReadRune()
		if err == io.EOF {
			return TOKEN_EOF
		} else if unicode.IsSpace(r) {
			if r == '\n' {
				lex.lineno++
			}
			continue
		}
		break
	}

	if r == '=' {
		for k := 0; k < 2; k++ { // check "==="
			r, _, err = lex.reader.ReadRune()
			if err == io.EOF {
				return TOKEN_EOF
			}
			if r != '=' {
				lex.reader.UnreadRune()
				return &token{typ: STRUCT_BEGIN}
			}
		}
		return &token{typ: STRUCT_END}
	} else if unicode.IsLetter(r) {
		var runes []rune
		for {
			runes = append(runes, r)
			r, _, err = lex.reader.ReadRune()
			if err == io.EOF {
				break
			} else if unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_' {
				continue
			} else {
				lex.reader.UnreadRune()
				break
			}
		}

		t := &token{}
		t.literal = string(runes)
		if datatypes[t.literal] {
			t.typ = DATA_TYPE
		} else if t.literal == "array" {
			t.typ = ARRAY_TYPE
		} else {
			t.typ = SYMBOL
		}

		return t
	} else {
		log.Fatal("lex error @line:", lex.lineno)
	}
	return nil
}

func (lex *Lexer) eof() bool {
	for {
		r, _, err := lex.reader.ReadRune()
		if err == io.EOF {
			return true
		} else if unicode.IsSpace(r) {
			if r == '\n' {
				lex.lineno++
			}
			continue
		} else {
			lex.reader.UnreadRune()
			return false
		}
	}
}

//////////////////////////////////////////////////////////////
type Parser struct {
	lexer *Lexer
	info  []struct_info
}

func (p *Parser) init(lex *Lexer) {
	p.lexer = lex
}

func (p *Parser) match(typ int) *token {
	t := p.lexer.next()
	if t.typ != typ {
		syntax_error(p)
	}
	return t
}

func (p *Parser) expr() bool {
	if p.lexer.eof() {
		return false
	}
	info := struct_info{}

	t := p.match(SYMBOL)
	info.Name = t.literal

	p.match(STRUCT_BEGIN)
	p.fields(&info)
	p.info = append(p.info, info)
	return true
}

func (p *Parser) fields(info *struct_info) {
	for {
		t := p.lexer.next()
		if t.typ == STRUCT_END {
			return
		}
		if t.typ != SYMBOL {
			syntax_error(p)
		}

		field := field_info{Name: t.literal}
		t = p.lexer.next()
		if t.typ == ARRAY_TYPE {
			field.Array = true
			t = p.match(SYMBOL)
			field.Typ = t.literal
		} else if t.typ == DATA_TYPE || t.typ == SYMBOL {
			field.Typ = t.literal
		} else {
			syntax_error(p)
		}

		info.Fields = append(info.Fields, field)
	}
}

func main() {

	if len(os.Args) != 2 {
		return
	}

	f, err := os.Open("func_map.json")
	if err != nil {
		log.Fatal(err)
	}
	dec := json.NewDecoder(f)

	// read open bracket
	_, err = dec.Token()
	if err != nil {
		log.Fatal(err)
	}

	for dec.More() {
		// decode an array value (Message)
		err := dec.Decode(&funcs)
		if err != nil {
			log.Fatal(err)
		}
	}

	// read closing bracket
	_, err = dec.Token()
	if err != nil {
		log.Fatal(err)
	}

	lexer := Lexer{}
	lexer.init(os.Stdin)
	p := Parser{}
	p.init(&lexer)
	for p.expr() {
	}

	log.Println(p.info)

	funcMap := template.FuncMap{
		"goType": func(t string) string {
			if v, ok := funcs[t]; ok {
				return v.Go.T
			} else {
				return ""
			}
		},
		"goRead": func(t string) string {
			if v, ok := funcs[t]; ok {
				return v.Go.R
			} else {
				return ""
			}
		},
		"goWrite": func(t string) string {
			if v, ok := funcs[t]; ok {
				return v.Go.W
			} else {
				return ""
			}
		},
		"csType": func(t string) string {
			if v, ok := funcs[t]; ok {
				return v.Cs.T
			} else {
				return ""
			}
		},
		"csRead": func(t string) string {
			if v, ok := funcs[t]; ok {
				return v.Cs.R
			} else {
				return ""
			}
		},
		"csWrite": func(t string) string {
			if v, ok := funcs[t]; ok {
				return v.Cs.W
			} else {
				return ""
			}
		},
	}
	tmpl, err := template.New("proto.tmpl").Funcs(funcMap).ParseFiles(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	err = tmpl.Execute(os.Stdout, p.info)
	if err != nil {
		log.Fatal(err)
	}
}
