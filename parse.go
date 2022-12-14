package flextime

import (
	"fmt"
	"strings"

	optionalstring "github.com/ngicks/flextime/optional_string"
)

type FormatError struct {
	idx      int
	expected string
	actual   string
	msg      string
}

func (e *FormatError) Error() string {
	return fmt.Sprintf("index [%d]: %s but %s. %s", e.idx, e.expected, e.actual, e.msg)
}

func ReplaceTimeTokenRaw(input optionalstring.RawString) (string, error) {
	var output string
	for _, vv := range input {
		switch vv.Typ() {
		case optionalstring.SingleQuoteEscaped, optionalstring.SlashEscaped:
			output += vv.Unescaped()
		case optionalstring.Normal:
			replaced, err := ReplaceTimeToken(vv.Unescaped())
			if err != nil {
				return "", err
			}
			output += string(replaced)
		}
	}
	return output, nil
}

func ReplaceTimeToken(input string) (string, error) {
	var prefix, token string
	var isToken bool
	var err error

	var output string

	for len(input) > 0 {
		prefix, token, input, isToken, err = nextChunk(input)
		if err != nil {
			return "", err
		}
		output += prefix
		if isToken {
			output += timeFormatToken(token).toGoFmt()
		} else {
			output += token
		}
	}

	return output, nil
}

// nextChunk reads input string from its head, up to a first time token or espaced string.
//
// prefix is non time token string which is read up before the first hit.
// found is next chunk string. If isTokein is true, chunk is a time token, an unescaped string otherwise.
// suffix is rest of input.
// err would be non nil if token has wrong length.
func nextChunk(input string) (prefix string, found string, suffix string, isToken bool, err error) {
	for i := 0; i < len(input); i++ {
		switch input[i] {
		case '\\':
			return input[:i], input[i+1 : i+2], input[i+2:], false, nil
		case '.':
			if strings.HasPrefix(input[i:], ".S") ||
				strings.HasPrefix(input[i:], ".9") ||
				strings.HasPrefix(input[i:], ".0") {
				repeated := getRepeatOf(input[i+1:], input[i+1:i+2])
				return input[:i], "." + repeated, input[i+len("."+repeated):], true, nil
			}
		case '\'':
			unescaped := getUntilClosingSingleQuote(input[i+1:])
			return input[:i], unescaped, input[i+len(`'`+unescaped+`'`):], false, nil
		}

		possibleSequences, ok := tokenSerachTable[input[i]]
		if ok {
			for _, possible := range possibleSequences {
				if strings.HasPrefix(string(input[i:]), string(possible)) {
					return input[:i], string(possible), input[i+len(possible):], true, nil
				}
			}
			if input[i] == '-' {
				continue
			}
			return "", "", "", false, &FormatError{
				idx:      i,
				expected: fmt.Sprintf("must be prefixed with one of %+v", possibleSequences),
				actual:   input[i:],
				msg:      "maybe wrong len, like Y or YYY.",
			}
		}
	}
	return input, "", "", false, nil
}

func getRepeatOf(input string, target string) string {
	for i := 0; i < len(input); i++ {
		if input[i:i+len(target)] != target {
			return input[:i+len(target)-1]
		}
	}
	return input
}

// getUntilClosingSingleQuote returns `aaaaa` if input is `aaaaa'`.
func getUntilClosingSingleQuote(input string) string {
	for i := 0; i < len(input); i++ {
		if input[i] == '\'' {
			if i == 0 {
				return ""
			}
			if input[i-1] != '\\' || strings.HasSuffix(input[:i+1], `\\'`) {
				return input[:i]
			}
		}
	}
	return input
}

var tokenSerachTable = map[byte][]timeFormatToken{
	'M': {"MMMM", "MMM", "MST", "MM", "M"},
	'w': {"ww", "w"},
	'd': {"ddd", "dd", "d"},
	'D': {"DDD", "DD", "D"},
	'H': {"HH"},
	'h': {"hh", "h"},
	'm': {"mm", "m"},
	's': {"ss", "s"},
	'Y': {"YYYY", "YY"},
	'y': {"yyyy", "yy"},
	'A': {"A"},
	'a': {"a"},
	'Z': {"Z07:00:00", "Z070000", "Z07", "ZZ", "Z"},
	// '-' with no successding 0 is non-token.
	'-': {"-07:00:00", "-070000", "-07:00", "-0700", "-07"},
	// '.' with suceeding 0,9,S needs special handling.
	// single '.' is non-token.
}

var tokenTable = map[timeFormatToken]goTimeFmtToken{
	"MMMM":      "January",
	"MMM":       "Jan",
	"M":         "1",
	"MM":        "01",
	"ww":        "Monday",
	"w":         "Mon",
	"D":         "2",
	"d":         "2",
	"DD":        "02",
	"dd":        "02",
	"DDD":       "002",
	"ddd":       "002",
	"HH":        "15",
	"h":         "3",
	"hh":        "03",
	"m":         "4",
	"mm":        "04",
	"s":         "5",
	"ss":        "05",
	"YYYY":      "2006",
	"yyyy":      "2006",
	"YY":        "06",
	"yy":        "06",
	"A":         "PM",
	"a":         "pm",
	"MST":       "MST",
	"ZZ":        "Z0700",
	"Z070000":   "Z070000",
	"Z07":       "Z07",
	"Z":         "Z07:00",
	"Z07:00:00": "Z07:00:00",
	"-0700":     "-0700",
	"-070000":   "-070000",
	"-07":       "-07",
	"-07:00":    "-07:00",
	"-07:00:00": "-07:00:00",
}

type timeFormatToken string

var tokens = [...]timeFormatToken{
	"MMMM",
	"MMM",
	"MM",
	"M",
	"ww",
	"w",
	"ddd",
	"dd",
	"d",
	"HH",
	"hh",
	"h",
	"mm",
	"m",
	"ss",
	"s",
	"YYYY",
	"YY",
	"A",
	"a",
	"MST",
	"Z07:00:00",
	"Z070000",
	"Z07",
	"ZZ",
	"Z",
	"-07:00:00",
	"-070000",
	"-07:00",
	"-0700",
	"-07",
	".S",
	".0",
	".9",
}

type goTimeFmtToken string

var goTimeFmtTokens = [...]goTimeFmtToken{
	"January",
	"Jan",
	"1",
	"01",
	"Monday",
	"Mon",
	"2",
	"02",
	"002",
	"15",
	"3",
	"03",
	"4",
	"04",
	"5",
	"05",
	"2006",
	"06",
	"PM",
	"pm",
	"MST",
	"Z0700",
	"Z070000",
	"Z07",
	"Z07:00",
	"Z07:00:00",
	"-0700",
	"-070000",
	"-07",
	"-07:00",
	"-07:00:00",
}

func (tt timeFormatToken) toGoFmt() string {
	token, ok := tokenTable[tt]
	if ok {
		return string(token)
	}

	if strings.HasPrefix(string(tt), ".S") {
		return strings.ReplaceAll(string(tt), "S", "0")
	} else if strings.HasPrefix(string(tt), ".0") || strings.HasPrefix(string(tt), ".9") {
		return string(tt)
	}
	panic(fmt.Sprintf("unknown: %s", tt))
}
