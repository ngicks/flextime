package flextime

import (
	"fmt"

	"github.com/ngicks/type-param-common/set"
	"github.com/ngicks/type-param-common/slice"
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
	return ast.Kleene(OPTIONALSTRING, nil, ast.OrdChoice("items", nil, optional, chars))
}

type SyntaxError struct {
	org      string
	parsedAs string
}

func (e SyntaxError) Error() string {
	return fmt.Sprintf(
		"syntax error: maybe no opening/closing sqrt? parsed result = %s, input = %s",
		e.parsedAs,
		e.org,
	)
}

func EnumerateOptionalString(optionalString string) (enumerated []string, err error) {
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
		return []string{}, &SyntaxError{
			org:      optionalString,
			parsedAs: parsedAs,
		}
	}

	root := &treeNode{}
	decode(node, root)

	f := root.Flatten()
	// This is what you want.
	//
	// out := make([]string, len(f))
	// for idx, v := range f {
	// 	out[idx] = v.String()
	// }

	// But this is it currently needs (returns some overlapping value.)
	// TODO: fix this.
	out := set.New[string]()
	for _, v := range f {
		out.Add(v.String())
	}

	return out.Values().Collect(), nil
}

type valueType int

const (
	normal valueType = iota
	singleQuoteEscaped
	slashEscaped
)

type value struct {
	typ   valueType
	value string
}

type treeNodeType int

const (
	text treeNodeType = iota
	optional
)

type treeNode struct {
	left  *treeNode
	right *treeNode
	value []value
	typ   treeNodeType
}

func (n *treeNode) Clone() []value {
	if n.value == nil {
		return nil
	}
	cloned := make([]value, len(n.value))
	copy(cloned, n.value)
	return cloned
}

func (n *treeNode) AddValue(v string, typ valueType) {
	n.value = append(n.value, value{value: v, typ: typ})
}

func (n *treeNode) SetAsOptional() {
	n.typ = optional
}

func (n *treeNode) IsOptional() bool {
	return n.typ == optional
}

func (n *treeNode) Left() *treeNode {
	if n.left == nil {
		n.left = &treeNode{}
	}
	return n.left
}
func (n *treeNode) HasLeft() bool {
	return n.left != nil
}

func (n *treeNode) Right() *treeNode {
	if n.right == nil {
		n.right = &treeNode{}
	}
	return n.right
}
func (n *treeNode) HasRight() bool {
	return n.right != nil
}

type rawString []value

func (rs rawString) Clone() rawString {
	cloned := make(rawString, len(rs))
	copy(cloned, rs)
	return cloned
}

func (rs rawString) Append(v rawString) rawString {
	return append(rs, v...)
}

func (rs rawString) String() string {
	var out string
	for _, v := range rs {
		out += v.value
	}
	return out
}

func (n *treeNode) Flatten() []rawString {
	// treeNodes is value of self -> left -> right order.
	var cur rawString
	var total []rawString
	if c := n.Clone(); len(c) > 0 {
		cur = rawString(c).Clone()
		total = []rawString{cur.Clone()}
	} else {
		total = []rawString{}
	}

	if n.HasLeft() {
		for _, s := range n.Left().Flatten() {
			total = append(total, cur.Clone().Append(s))
		}
	}

	if n.HasRight() {
		r := n.Right().Flatten()
		_, hasNonEmpty := slice.Find(r, func(rs rawString) bool { return rs.String() != "" })
		if hasNonEmpty {
			org := make([]rawString, len(total))
			copy(org, total)
			total = total[:0]
			for _, rws := range r {
				for _, oo := range org {
					total = append(total, oo.Clone().Append(rws))
				}
			}
		}
	}

	if n.IsOptional() && cur.String() != "" {
		// if optional, add string that does not contain nodes below.
		total = append(total, rawString{})
	}

	return total
}

func decode(node parsec.Queryable, root *treeNode) {
	recursiveDecode(node.GetChildren(), root)
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
			recursiveDecode(nodes, ctx)
		case OPTIONAL:
			var optNext *treeNode
			if !onceFound {
				onceFound = true
				optNext = ctx.Left()
			} else {
				optNext = ctx.Right()
			}
			optNext.SetAsOptional()
			recursiveDecode(nodes[i].GetChildren(), optNext)
		case CHARS:
			for _, v := range nodes[i].GetChildren() {
				switch v.GetName() {
				case NORMALCHARS:
					ctx.AddValue(v.GetValue(), normal)
				case ESCAPEDCHAR:
					ctx.AddValue(v.GetValue(), singleQuoteEscaped)
				default:
					panic(fmt.Sprintf("incorrect implementation: %s, %s", v.GetName(), v.GetValue()))
				}
			}
		case ESCAPED:
			ctx.AddValue(nodes[i].GetValue(), singleQuoteEscaped)
		case ITEMS:
			recursiveDecode(nodes[i].GetChildren(), ctx)
		}
	}
}
