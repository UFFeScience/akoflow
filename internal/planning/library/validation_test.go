package library

import (
	"context"
	"strings"
	"testing"
)

func TestSyntaxValidationUsesOriginalPositionsAndFormats(t *testing.T) {
	source := "func Schedule(input Input) ([]Plan, error) {\nreturn nil, nil\n}"
	checked, err := Validate(context.Background(), source, false)
	if err != nil || !checked.Valid || checked.Compiled || !strings.Contains(checked.FormattedSource, "\treturn nil, nil") {
		t.Fatalf("result=%+v error=%v", checked, err)
	}
	if strings.Contains(checked.FormattedSource, "package main") {
		t.Fatal("formatter introduced hidden package declaration")
	}
	broken, err := Validate(context.Background(), "func Schedule(input Input) ([]Plan, error) {\nreturn ???\n}", false)
	if err != nil || broken.Valid || len(broken.Diagnostics) == 0 || broken.Diagnostics[0].Line != 2 {
		t.Fatalf("broken=%+v error=%v", broken, err)
	}
}
func TestSignatureValidation(t *testing.T) {
	for _, source := range []string{
		"package other\nfunc Schedule(input Input) ([]Plan,error){return nil,nil}",
		"func Another(input Input)([]Plan,error){return nil,nil}",
		"func Schedule(input string)([]Plan,error){return nil,nil}",
		"func Schedule(input Input)(Plan,error){return Plan{},nil}",
		"func Schedule(input Input)([]Plan,error){return nil,nil}\nfunc main(){}",
	} {
		checked, err := Validate(context.Background(), source, false)
		if err != nil || checked.Valid || len(checked.Diagnostics) == 0 {
			t.Fatalf("accepted invalid contract %q: %+v %v", source, checked, err)
		}
	}
}
func TestCompilerValidationReportsTypeErrorsWithoutRegistration(t *testing.T) {
	source := "func Schedule(input Input) ([]Plan, error) {\n return missingVariable, nil\n}"
	syntax, err := Validate(context.Background(), source, false)
	if err != nil || !syntax.Valid {
		t.Fatalf("syntax=%+v err=%v", syntax, err)
	}
	checked, err := Validate(context.Background(), source, true)
	if err != nil || checked.Valid || len(checked.Diagnostics) == 0 || checked.Diagnostics[0].Line != 2 || !strings.Contains(checked.Diagnostics[0].Message, "missingVariable") {
		t.Fatalf("type check=%+v error=%v", checked, err)
	}
	valid, err := Validate(context.Background(), Template, true)
	if err != nil || !valid.Valid || !valid.Compiled {
		t.Fatalf("template=%+v error=%v", valid, err)
	}
}
