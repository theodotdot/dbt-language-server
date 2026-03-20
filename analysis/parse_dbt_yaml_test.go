package analysis

import (
	"fmt"
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
					},
					{
						Name: AnnotatedField[string]{
							Value:    "customers",
							Position: lsp.Position{Line: 89, Character: 14},
						},
						Description: AnnotatedField[string]{
							Value:    "",
							Position: lsp.Position{Line: 0, Character: 0},
						},
					},
				},
			},
			{
				Name: AnnotatedField[string]{
					Value:    "stripe",
					Position: lsp.Position{Line: 91, Character: 10},
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
							Position: lsp.Position{Line: 93, Character: 14},
						},
						Description: AnnotatedField[string]{
							Value:    "",
							Position: lsp.Position{Line: 0, Character: 0},
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
