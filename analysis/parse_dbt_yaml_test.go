package analysis

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/j-clemons/dbt-language-server/lsp"
	"github.com/j-clemons/dbt-language-server/testutils"
)

func TestParsePropertiesYamlFile(t *testing.T) {
	testdataRoot, err := testutils.GetTestdataPath("jaffle_shop_duckdb")
	if err != nil {
		panic(err)
	}

	actualProperties := parsePropertiesYamlFile(
		filepath.Join(testdataRoot, "models/schema.yml"),
	)

	expectedProperties := PropertiesYaml{
		Models: []ModelProperties{
			{
				Name: AnnotatedField[string]{
					Value: "customers", Position: lsp.Position{Line: 3, Character: 10},
				},
				Description: AnnotatedField[string]{
					Value:    "This table has basic information about a customer, as well as some derived facts based on a customer's orders",
					Position: lsp.Position{Line: 4, Character: 17},
				},
				ModelConfig: AnnotatedMap(nil),
				Columns: []ColumnProperties{
					{Name: AnnotatedField[string]{Value: "customer_id", Position: lsp.Position{Line: 7, Character: 14}}, Description: AnnotatedField[string]{Value: "This is a unique identifier for a customer", Position: lsp.Position{Line: 8, Character: 21}}},
					{Name: AnnotatedField[string]{Value: "first_name", Position: lsp.Position{Line: 13, Character: 14}}, Description: AnnotatedField[string]{Value: "Customer's first name. PII.", Position: lsp.Position{Line: 14, Character: 21}}},
					{Name: AnnotatedField[string]{Value: "last_name", Position: lsp.Position{Line: 16, Character: 14}}, Description: AnnotatedField[string]{Value: "Customer's last name. PII.", Position: lsp.Position{Line: 17, Character: 21}}},
					{Name: AnnotatedField[string]{Value: "first_order", Position: lsp.Position{Line: 19, Character: 14}}, Description: AnnotatedField[string]{Value: "Date (UTC) of a customer's first order", Position: lsp.Position{Line: 20, Character: 21}}},
					{Name: AnnotatedField[string]{Value: "most_recent_order", Position: lsp.Position{Line: 22, Character: 14}}, Description: AnnotatedField[string]{Value: "Date (UTC) of a customer's most recent order", Position: lsp.Position{Line: 23, Character: 21}}},
					{Name: AnnotatedField[string]{Value: "number_of_orders", Position: lsp.Position{Line: 25, Character: 14}}, Description: AnnotatedField[string]{Value: "Count of the number of orders a customer has placed", Position: lsp.Position{Line: 26, Character: 21}}},
					{Name: AnnotatedField[string]{Value: "total_order_amount", Position: lsp.Position{Line: 28, Character: 14}}, Description: AnnotatedField[string]{Value: "Total value (AUD) of a customer's orders", Position: lsp.Position{Line: 29, Character: 21}}},
				},
			},
			{
				Name: AnnotatedField[string]{
					Value:    "orders",
					Position: lsp.Position{Line: 31, Character: 10},
				},
				Description: AnnotatedField[string]{
					Value:    "This table has basic information about orders, as well as some derived facts based on payments",
					Position: lsp.Position{Line: 32, Character: 17},
				},
				ModelConfig: AnnotatedMap(nil),
				Columns: []ColumnProperties{
					{Name: AnnotatedField[string]{Value: "order_id", Position: lsp.Position{Line: 35, Character: 14}}, Description: AnnotatedField[string]{Value: "This is a unique identifier for an order", Position: lsp.Position{Line: 39, Character: 21}}},
					{Name: AnnotatedField[string]{Value: "customer_id", Position: lsp.Position{Line: 41, Character: 14}}, Description: AnnotatedField[string]{Value: "Foreign key to the customers table", Position: lsp.Position{Line: 42, Character: 21}}},
					{Name: AnnotatedField[string]{Value: "order_date", Position: lsp.Position{Line: 49, Character: 14}}, Description: AnnotatedField[string]{Value: "Date (UTC) that the order was placed", Position: lsp.Position{Line: 50, Character: 21}}},
					{Name: AnnotatedField[string]{Value: "status", Position: lsp.Position{Line: 52, Character: 14}}, Description: AnnotatedField[string]{Value: `{{ doc("orders_status") }}`, Position: lsp.Position{Line: 53, Character: 21}}},
					{Name: AnnotatedField[string]{Value: "amount", Position: lsp.Position{Line: 58, Character: 14}}, Description: AnnotatedField[string]{Value: "Total amount (AUD) of the order", Position: lsp.Position{Line: 59, Character: 21}}},
					{Name: AnnotatedField[string]{Value: "credit_card_amount", Position: lsp.Position{Line: 63, Character: 14}}, Description: AnnotatedField[string]{Value: "Amount of the order (AUD) paid for by credit card", Position: lsp.Position{Line: 64, Character: 21}}},
					{Name: AnnotatedField[string]{Value: "coupon_amount", Position: lsp.Position{Line: 68, Character: 14}}, Description: AnnotatedField[string]{Value: "Amount of the order (AUD) paid for by coupon", Position: lsp.Position{Line: 69, Character: 21}}},
					{Name: AnnotatedField[string]{Value: "bank_transfer_amount", Position: lsp.Position{Line: 73, Character: 14}}, Description: AnnotatedField[string]{Value: "Amount of the order (AUD) paid for by bank transfer", Position: lsp.Position{Line: 74, Character: 21}}},
					{Name: AnnotatedField[string]{Value: "gift_card_amount", Position: lsp.Position{Line: 78, Character: 14}}, Description: AnnotatedField[string]{Value: "Amount of the order (AUD) paid for by gift card", Position: lsp.Position{Line: 79, Character: 21}}},
				},
			},
		},
		Sources: []SourceProperties{
			{
				Name: AnnotatedField[string]{
					Value:    "jaffle_shop",
					Position: lsp.Position{Line: 84, Character: 10},
				},
				Database: AnnotatedField[string]{
					Value:    "raw",
					Position: lsp.Position{Line: 85, Character: 14},
				},
				Schema: AnnotatedField[string]{
					Value:    "jaffle_shop",
					Position: lsp.Position{Line: 86, Character: 12},
				},
				Description: AnnotatedField[string]{
					Value:    "",
					Position: lsp.Position{Line: 0, Character: 0},
				},
				Tables: []SourceTableProperties{
					{
						Name: AnnotatedField[string]{
							Value:    "orders",
							Position: lsp.Position{Line: 88, Character: 14},
						},
						Description: AnnotatedField[string]{
							Value:    "",
							Position: lsp.Position{Line: 0, Character: 0},
						},
						Columns: []ColumnProperties{
							{Name: AnnotatedField[string]{Value: "order_id", Position: lsp.Position{Line: 90, Character: 18}}, Description: AnnotatedField[string]{Value: "", Position: lsp.Position{Line: 91, Character: 25}}},
							{Name: AnnotatedField[string]{Value: "customer_id", Position: lsp.Position{Line: 92, Character: 18}}, Description: AnnotatedField[string]{Value: "", Position: lsp.Position{Line: 93, Character: 25}}},
							{Name: AnnotatedField[string]{Value: "order_date", Position: lsp.Position{Line: 94, Character: 18}}, Description: AnnotatedField[string]{Value: "", Position: lsp.Position{Line: 95, Character: 25}}},
						},
					},
					{
						Name: AnnotatedField[string]{
							Value:    "customers",
							Position: lsp.Position{Line: 96, Character: 14},
						},
						Description: AnnotatedField[string]{
							Value:    "",
							Position: lsp.Position{Line: 0, Character: 0},
						},
						Columns: []ColumnProperties{
							{Name: AnnotatedField[string]{Value: "customer_id", Position: lsp.Position{Line: 98, Character: 18}}, Description: AnnotatedField[string]{Value: "", Position: lsp.Position{Line: 99, Character: 25}}},
							{Name: AnnotatedField[string]{Value: "first_name", Position: lsp.Position{Line: 100, Character: 18}}, Description: AnnotatedField[string]{Value: "", Position: lsp.Position{Line: 101, Character: 25}}},
							{Name: AnnotatedField[string]{Value: "last_name", Position: lsp.Position{Line: 102, Character: 18}}, Description: AnnotatedField[string]{Value: "", Position: lsp.Position{Line: 103, Character: 25}}},
						},
					},
				},
			},
			{
				Name: AnnotatedField[string]{
					Value:    "stripe",
					Position: lsp.Position{Line: 105, Character: 10},
				},
				Database: AnnotatedField[string]{
					Value:    "",
					Position: lsp.Position{Line: 0, Character: 0},
				},
				Schema: AnnotatedField[string]{
					Value:    "",
					Position: lsp.Position{Line: 0, Character: 0},
				},
				Description: AnnotatedField[string]{
					Value:    "",
					Position: lsp.Position{Line: 0, Character: 0},
				},
				Tables: []SourceTableProperties{
					{
						Name: AnnotatedField[string]{
							Value:    "payments",
							Position: lsp.Position{Line: 107, Character: 14},
						},
						Description: AnnotatedField[string]{
							Value:    "",
							Position: lsp.Position{Line: 0, Character: 0},
						},
						Columns: []ColumnProperties{
							{Name: AnnotatedField[string]{Value: "payment_id", Position: lsp.Position{Line: 109, Character: 18}}, Description: AnnotatedField[string]{Value: "Unique payment identifier", Position: lsp.Position{Line: 110, Character: 25}}},
							{Name: AnnotatedField[string]{Value: "payment_method", Position: lsp.Position{Line: 111, Character: 18}}, Description: AnnotatedField[string]{Value: "", Position: lsp.Position{Line: 112, Character: 25}}},
							{Name: AnnotatedField[string]{Value: "amount", Position: lsp.Position{Line: 113, Character: 18}}, Description: AnnotatedField[string]{Value: "", Position: lsp.Position{Line: 114, Character: 25}}},
						},
					},
				},
			},
		},
	}

	if fmt.Sprintf("%#v", actualProperties) != fmt.Sprintf("%#v", expectedProperties) {
		t.Errorf("expected %#v but got %#v", expectedProperties, actualProperties)
	}
}

func TestSourceTableColumnsParsing(t *testing.T) {
	t.Run("source with columns", func(t *testing.T) {
		yml := `
sources:
  - name: my_src
    tables:
      - name: my_tbl
        columns:
          - name: col_a
            description: "First column"
          - name: col_b
            description: ""
`
		tmpFile := filepath.Join(t.TempDir(), "test.yml")
		if err := os.WriteFile(tmpFile, []byte(yml), 0644); err != nil {
			t.Fatal(err)
		}
		props := parsePropertiesYamlFile(tmpFile)
		if len(props.Sources) != 1 {
			t.Fatalf("expected 1 source, got %d", len(props.Sources))
		}
		cols := props.Sources[0].Tables[0].Columns
		if len(cols) != 2 {
			t.Fatalf("expected 2 columns, got %d", len(cols))
		}
		if cols[0].Name.Value != "col_a" || cols[0].Description.Value != "First column" {
			t.Errorf("col_a: got name=%q desc=%q", cols[0].Name.Value, cols[0].Description.Value)
		}
		if cols[1].Name.Value != "col_b" || cols[1].Description.Value != "" {
			t.Errorf("col_b: got name=%q desc=%q", cols[1].Name.Value, cols[1].Description.Value)
		}
	})

	t.Run("source without columns", func(t *testing.T) {
		yml := `
sources:
  - name: my_src
    tables:
      - name: bare_tbl
`
		tmpFile := filepath.Join(t.TempDir(), "test.yml")
		if err := os.WriteFile(tmpFile, []byte(yml), 0644); err != nil {
			t.Fatal(err)
		}
		props := parsePropertiesYamlFile(tmpFile)
		cols := props.Sources[0].Tables[0].Columns
		if cols != nil {
			t.Errorf("expected nil columns for bare table, got %v", cols)
		}
	})
}
