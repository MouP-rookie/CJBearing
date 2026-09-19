package service

import "testing"

func TestResolveDefaultTokens(t *testing.T) {
	got := ResolveDefault("{{user_name}}/{{line_name}}/{{line_product}}/{{stage}}", DefaultContext{UserName: "张三", LineName: "磨工1线", LineProduct: "6203-2RS", Stage: "首检"})
	if got != "张三/磨工1线/6203-2RS/首检" {
		t.Fatalf("unexpected resolved value: %s", got)
	}
}

func TestValidateSchemaRejectsDuplicateKey(t *testing.T) {
	schema := FormSchema{Fields: []FieldDefinition{{Key: "x", Label: "A", Type: "text"}, {Key: "x", Label: "B", Type: "text"}}}
	if err := ValidateSchema(schema); err == nil {
		t.Fatal("expected duplicate key error")
	}
}
func TestValidateSchemaAcceptsProductField(t *testing.T) {
	schema := FormSchema{Fields: []FieldDefinition{{Key: "product", Label: "产品", Type: "product", Required: true}}}
	if err := ValidateSchema(schema); err != nil {
		t.Fatalf("expected product field accepted, got %v", err)
	}
}

func TestValidateSchemaRejectsUnknownType(t *testing.T) {
	schema := FormSchema{Fields: []FieldDefinition{{Key: "x", Label: "X", Type: "product_multi"}}}
	if err := ValidateSchema(schema); err == nil {
		t.Fatal("expected unknown type error")
	}
}

func TestValidateSchemaRejectsProductMultiValue(t *testing.T) {
	schema := FormSchema{Fields: []FieldDefinition{{Key: "product", Label: "产品", Type: "product", AllowMultiple: true}}}
	if err := ValidateSchema(schema); err == nil {
		t.Fatal("expected product multi value error")
	}
}

func TestPrependProductFieldPrependsAndShiftsOrder(t *testing.T) {
	schema := FormSchema{Fields: []FieldDefinition{{Key: "a", Label: "外径", Type: "number", SortOrder: 0}, {Key: "b", Label: "内径", Type: "number", SortOrder: 1}}}
	next := PrependProductField(schema)
	if len(next.Fields) != 3 || next.Fields[0].Type != "product" || next.Fields[0].Key != "product" || !next.Fields[0].Required {
		t.Fatalf("unexpected prepended schema: %+v", next)
	}
	if next.Fields[1].SortOrder != 1 || next.Fields[2].SortOrder != 2 {
		t.Fatalf("sort order not shifted: %+v", next.Fields)
	}
}

func TestPrependProductFieldKeyConflict(t *testing.T) {
	schema := FormSchema{Fields: []FieldDefinition{{Key: "product", Label: "旧产品", Type: "text"}}}
	next := PrependProductField(schema)
	if next.Fields[0].Key != "product_2" {
		t.Fatalf("expected product_2 key, got %s", next.Fields[0].Key)
	}
	if HasProductField(schema) {
		t.Fatal("text field named product is not a product field")
	}
	if !HasProductField(next) {
		t.Fatal("prepended schema should report product field")
	}
}
