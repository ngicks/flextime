package optionalstring

import (
	"fmt"

	"github.com/pkg/errors"
	parsec "github.com/prataprc/goparsec"
)

const (
	OPENSQR           = "OPENSQR"
	CLOSESQR          = "CLOSESQR"
	SQUOTE            = "SQUOTE"
	ESCAPEDCHAR       = "ESCAPEDCHAR"
	NORMALCHARS       = "NORMALCHARS"
	CHAR              = "CHAR"
	CHARS             = "CHARS"
	CHARWITHINESCAPE  = "CHARWITHINESCAPE"
	CHARSWITHINESCAPE = "CHARSWITHINESCAPE"
	ESCAPED           = "ESCAPED"
	ITEM              = "ITEM"
	ITEMS             = "ITEMS"
	OPTIONAL          = "OPTIONAL"
	OPTIONALSTRING    = "OPTIONALSTRING"
)

var (
	opensqr     parsec.Parser = parsec.Atom(`[`, OPENSQR)
	closesqr                  = parsec.Atom(`]`, CLOSESQR)
	squote                    = parsec.Atom(`'`, SQUOTE)
	escapedchar               = parsec.Token(`\\.`, ESCAPEDCHAR)
	normalchars               = parsec.Token(`[^\[\]\\']+`, NORMALCHARS)
)

func MakeOptionalStringParser(ast *parsec.AST) parsec.Parser {
	char := ast.OrdChoice(CHAR, nil, escapedchar, normalchars)
	chars := ast.Many(CHARS, nil, char)
	charWithinEscape := ast.OrdChoice(CHARWITHINESCAPE, nil, escapedchar, normalchars, opensqr, closesqr)
	charsWithinEscape := ast.Many(CHARSWITHINESCAPE, nil, charWithinEscape)

	var optional parsec.Parser
	escaped := ast.And(ESCAPED, nil, squote, charsWithinEscape, squote)
	item := ast.OrdChoice(ITEM, nil, chars, escaped, &optional)
	items := ast.Kleene(ITEMS, nil, item)
	optional = ast.And(OPTIONAL, nil, opensqr, items, closesqr)
	return ast.Kleene(OPTIONALSTRING, nil, ast.OrdChoice("items", nil, optional, item))
}

type SyntaxError struct {
	Input    string
	ParsedAs string
}

func (e SyntaxError) Error() string {
	return fmt.Sprintf(
		"syntax error: maybe no opening/closing sqrt? parsed result = %s, input = %s",
		e.ParsedAs,
		e.Input,
	)
}

func EnumerateOptionalStringRaw(optionalString string) (enumerated []RawString, err error) {
	var node parsec.Queryable
	func() {
		defer func() {
			if rcv := recover(); rcv != nil {
				err = errors.Errorf("%+v", rcv)
			}
		}()

		ast := parsec.NewAST("optionalString", 100)
		p := MakeOptionalStringParser(ast)
		s := parsec.NewScanner([]byte(optionalString))
		node, _ = ast.Parsewith(p, s)
	}()

	if err != nil {
		return
	}

	if parsedAs := node.GetValue(); len(parsedAs) != len(optionalString) {
		return []RawString{}, &SyntaxError{
			Input:    optionalString,
			ParsedAs: parsedAs,
		}
	}

	root := decode(node)

	return root.Flatten(), nil
}

func EnumerateOptionalString(optionalString string) (enumerated []string, err error) {
	raw, err := EnumerateOptionalStringRaw(optionalString)
	if err != nil {
		return []string{}, err
	}

	out := make([]string, len(raw))
	for idx, v := range raw {
		out[idx] = v.String()
	}
	return out, nil
}

func decode(node parsec.Queryable) *treeNode {
	root := &treeNode{}
	recursiveDecode(node.GetChildren(), root)
	return root
}

func recursiveDecode(nodes []parsec.Queryable, ctx *treeNode) {
	var onceFound bool

	for i := 0; i < len(nodes); i++ {
		if onceFound {
			recursiveDecode(nodes[i:], ctx.Right())
			return
		}

		switch nodes[i].GetName() {
		case OPTIONALSTRING:
			// skipping first node.
			recursiveDecode(nodes[i].GetChildren(), ctx)
		case OPTIONAL:
			var optNext *treeNode
			if !onceFound {
				onceFound = true
				optNext = ctx.Left()
			} else {
				panic(
					fmt.Sprintf(
						"incorrect implementation: %s, %s",
						nodes[i].GetName(),
						nodes[i].GetValue(),
					),
				)
			}
			optNext.SetAsOptional()
			recursiveDecode(nodes[i].GetChildren(), optNext)
		case CHARS:
			for _, v := range nodes[i].GetChildren() {
				switch v.GetName() {
				case NORMALCHARS:
					ctx.AddValue(v.GetValue(), Normal)
				case ESCAPEDCHAR:
					ctx.AddValue(v.GetValue(), SingleQuoteEscaped)
				default:
					panic(fmt.Sprintf("incorrect implementation: %s, %s", v.GetName(), v.GetValue()))
				}
			}
		case ESCAPED:
			ctx.AddValue(nodes[i].GetValue(), SingleQuoteEscaped)
		case ITEMS:
			recursiveDecode(nodes[i].GetChildren(), ctx)
		}
	}
}
