// * last modified: 26/11/19
package calculate

import (
	"strings"
	"testing"
)

// TestCalc is the main table-driven regression suite for Calc.
//
// The first block covers everyday arithmetic and the function set. The blocks
// tagged A01-F08 mirror the boundary report (report-20261118-204525.md) case
// by case, so a failing name points straight at the report entry it guards:
//
//	A - output range: results beyond int64 must not be clamped
//	B - "^" precedence, right associativity and unary minus
//	C - decimal-only literals; Go-style forms are rejected
//	D - exact arity for fixed-argument functions
//	E - NaN and Inf are errors, never results
//	F - integer precision through min, max, abs and mixed arithmetic
//
// A row with want == "" and wantErr == false only asserts that evaluation
// succeeds, for results whose exact digits are not part of the contract
// (for example sqrt(2)).
func TestCalc(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		want    string
		wantErr bool
	}{
		// input validation: empty, whitespace-only and malformed expressions
		{"empty", "", "", true},
		{"blank", "   ", "", true},
		{"invalid syntax", "(", "", true},
		// integer arithmetic stays exact; / and % truncate toward zero
		{"add ints", "2+3", "5", false},
		{"sub ints", "10-3", "7", false},
		{"mul ints", "4*5", "20", false},
		{"div ints exact", "10/2", "5", false},
		{"div ints truncated", "10/3", "3", false},
		{"mod ints", "10%3", "1", false},
		{"div by zero int", "5/0", "", true},
		{"mod by zero int", "5%0", "", true},
		{"power int", "2^3", "8", false},
		// float arithmetic; whole float results print without a decimal point
		{"add floats", "1.5+2.5", "4", false},
		{"div float result", "5.0/2.0", "2.5", false},
		{"div float exact", "6.0/2.0", "3", false},
		{"div by zero float", "5.0/0", "", true},
		{"mod by zero float", "5.0%0", "", true},
		{"mixed add", "1+0.5", "1.5", false},
		// unary signs, including -0.0 normalised to "0"
		{"unary neg int", "-5", "-5", false},
		{"unary pos int", "+5", "5", false},
		{"unary neg float", "-1.5", "-1.5", false},
		{"negative zero", "-0.0", "0", false},
		// grouping, whitespace tolerance and big.Int multiplication
		{"paren", "(2+3)*4", "20", false},
		{"nested paren", "((2+3)*2)+1", "11", false},
		{"spaces", " ( 100 + 200 ) * 3 ", "900", false},
		{"big int", "1000000000*1000000000", "1000000000000000000", false},
		// built-in functions and their domain errors; rounding an integer keeps it exact
		{"sqrt perfect", "sqrt(4)", "2", false},
		{"sqrt float", "sqrt(2)", "", false},
		{"sqrt negative", "sqrt(-1)", "", true},
		{"abs negative", "abs(-5)", "5", false},
		{"abs positive", "abs(3)", "3", false},
		{"abs float", "abs(-1.5)", "1.5", false},
		{"ceil", "ceil(1.2)", "2", false},
		{"floor", "floor(1.9)", "1", false},
		{"round up", "round(1.5)", "2", false},
		{"round down", "round(1.4)", "1", false},
		{"round int", "round(9007199254740993)", "9007199254740993", false},
		{"log of 1", "log(1)", "0", false},
		{"log zero", "log(0)", "", true},
		{"log negative", "log(-1)", "", true},
		{"log2 of 8", "log2(8)", "3", false},
		{"log10 of 100", "log10(100)", "2", false},
		{"sin 0", "sin(0)", "0", false},
		{"cos 0", "cos(0)", "1", false},
		{"tan 0", "tan(0)", "0", false},
		{"pow 2 args", "pow(2,3)", "8", false},
		{"pow 1 arg", "pow(2)", "", true},
		{"unknown fn", "unknown(5)", "", true},

		// A: values past int64 keep their magnitude; huge powers fall back to float and overflow to an error
		{"A01 2^63", "2^63", "9223372036854775808", false},
		{"A02 2^64", "2^64", "18446744073709551616", false},
		{"A03 1e19", "1e19", "10000000000000000000", false},
		{"A04 1e20", "1e20", "100000000000000000000", false},
		{"A05 pow(10,20)", "pow(10,20)", "100000000000000000000", false},
		{"A06 min big floats", "min(1e20,2e20)", "100000000000000000000", false},
		{"A08 -1e20", "-1e20", "-100000000000000000000", false},
		{"A09 int64 overflow literal", "9223372036854775808", "9223372036854775808", false},
		{"A10 int64 min float", "-9223372036854775808.0", "-9223372036854776000", false},
		{"A11 2^53 float", "9007199254740992.0", "9007199254740992", false},
		{"pow huge exponent", "2^100000", "", true},
		{"pow base one huge exponent", "1^1000000000", "1", false},

		// B: ^ binds tighter than * / + - and unary minus, and groups right to left
		{"B01 add then power", "2+3^2", "11", false},
		{"B02 sub then power", "10-2^3", "2", false},
		{"B03 mul then power", "2*3^2", "18", false},
		{"B04 div then power", "16/2^2", "4", false},
		{"B05 right assoc", "2^3^2", "512", false},
		{"B06 unary minus", "-2^2", "-4", false},
		{"B07 grouped", "2+(3^2)", "11", false},
		{"B08 grouped exponent", "2^(3^2)", "512", false},
		{"B09 negative exponent", "2^-2", "0.25", false},
		{"paren negative base", "(-2)^2", "4", false},

		// C: only plain decimal literals; leading zeros are decimal, not octal
		{"C01 underscore int", "1_000", "", true},
		{"C02 underscore float", "1_000.5", "", true},
		{"C03 hex", "0x10", "", true},
		{"C04 binary", "0b1010", "", true},
		{"C05 octal", "0o10", "", true},
		{"C06 hex float", "0x1.8p1", "", true},
		{"C07 leading zero", "010", "10", false},
		{"C08 leading zero 8", "08", "8", false},
		{"leading dot", ".5+1", "1.5", false},

		// D: extra or missing arguments are errors instead of being ignored
		{"D01 log extra", "log(100,10)", "", true},
		{"D02 sqrt extra", "sqrt(4,9)", "", true},
		{"D03 abs extra", "abs(-5,99)", "", true},
		{"D04 pow extra", "pow(2,3,4)", "", true},
		{"D05 sin extra", "sin(0,1)", "", true},
		{"D06 pow missing", "pow(2)", "", true},
		{"D07 sqrt empty", "sqrt()", "", true},

		// E: any NaN or Inf, whether produced by a function, an operator or a literal, is rejected
		{"E01 pow nan", "pow(-2,0.5)", "", true},
		{"E02 sqrt of nan", "sqrt(pow(-1,0.5))", "", true},
		{"E03 log of nan", "log(pow(-1,0.5))", "", true},
		{"E04 min with nan", "min(1,pow(-1,0.5))", "", true},
		{"E05 pow inf", "pow(0,-1)", "", true},
		{"E06 overflow mul", "1e308*10", "", true},
		{"E07 overflow int literal mixed", "1" + strings.Repeat("0", 310) + "+0.0", "", true},
		{"E08 inf minus inf", "(1e308*10)-(1e308*10)", "", true},
		{"E09 sqrt negative", "sqrt(-1)", "", true},
		{"E10 overflow float literal", "1e309", "", true},

		// F: min/max return the winning operand unchanged and abs keeps integers exact;
		// mixing in a float deliberately rounds to float64 (F02)
		{"F01 max mixed keeps int", "max(1000000000000000001,1.5)", "1000000000000000001", false},
		{"F02 mixed rounds to float64", "9007199254740993+0.0", "9007199254740992", false},
		{"F03 int path exact", "9007199254740993+0", "9007199254740993", false},
		{"F04 max ints", "max(1000000000000000001,1)", "1000000000000000001", false},
		{"F05 min three", "min(3,2,1)", "1", false},
		{"F06 max negatives", "max(-3,-2,-1)", "-1", false},
		{"F07 min empty", "min()", "", true},
		{"F08 abs big int", "abs(-9007199254740993)", "9007199254740993", false},
		{"min mixed keeps float", "min(2,1.5)", "1.5", false},
		{"max single", "max(7)", "7", false},
	}

	// each row runs as a named subtest so a failure reports the case name
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Calc(tt.expr)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Calc(%q) = %q, error = %v, wantErr %v", tt.expr, got, err, tt.wantErr)
			}
			if !tt.wantErr && tt.want != "" && got != tt.want {
				t.Errorf("Calc(%q) = %q, want %q", tt.expr, got, tt.want)
			}
		})
	}
}

// TestCalc_LargeFinite checks the output format for a finite float far past
// int64 (report A07): the value prints in plain decimal using the shortest
// round-trip digits, so 1e308 is "1" followed by 308 zeros rather than
// scientific notation or the float's exact binary expansion.
func TestCalc_LargeFinite(t *testing.T) {
	got, err := Calc("1e308")
	if want := "1" + strings.Repeat("0", 308); err != nil || got != want {
		t.Errorf("Calc(\"1e308\") = %q, %v; want 1 followed by 308 zeros", got, err)
	}
}

// TestCalc_ErrorMessages pins the wording of agent-facing errors. TestCalc
// only checks that an error occurred; this test makes sure each failure class
// (missing arguments, wrong arity, non-finite result, invalid literal, bare
// name, stray operator) still says what went wrong.
func TestCalc_ErrorMessages(t *testing.T) {
	tests := []struct {
		expr string
		want string
	}{
		{"sqrt()", "requires arguments"},
		{"pow(2)", "requires 2 arguments, got 1"},
		{"pow(2,3,4)", "requires 2 arguments, got 3"},
		{"log(100,10)", "requires 1 argument, got 2"},
		{"pow(-2,0.5)", "not a finite number"},
		{"0x10", "invalid number: 0x10"},
		{"x", "unknown name"},
		{"2|3", "unexpected '|'"},
	}
	for _, tt := range tests {
		_, err := Calc(tt.expr)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("Calc(%q) error = %v, want containing %q", tt.expr, err, tt.want)
		}
	}
}

// TestCalc_UnsupportedSyntax lists inputs the parser must reject: characters
// outside the grammar ('a', ^ as a prefix, |), qualified or bare names,
// errors raised inside operands and arguments, juxtaposed values with no
// operator, unbalanced or malformed calls, and a lone ".".
func TestCalc_UnsupportedSyntax(t *testing.T) {
	for _, expr := range []string{
		"'a'",
		"^5",
		"2|3",
		"1.5|2.5",
		"math.Sqrt(4)",
		"x",
		"sqrt(x)",
		"x+2",
		"2+x",
		"-(x)",
		"2 3",
		"2(3)",
		"sqrt(4",
		"sqrt(4;",
		"pow(2,)",
		".",
	} {
		if _, err := Calc(expr); err == nil {
			t.Errorf("Calc(%q) expected error", expr)
		}
	}
}
