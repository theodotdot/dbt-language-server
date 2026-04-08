package analysis

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/j-clemons/dbt-language-server/docs"
	"github.com/j-clemons/dbt-language-server/lsp"
	"github.com/j-clemons/dbt-language-server/testutils"
)

func expectedTestState() State {
	testdataRoot, err := testutils.GetTestdataPath("jaffle_shop_duckdb")
	if err != nil {
		panic(err)
	}

	expectedState := State{
		Documents: map[string]Document{},
		DbtContext: DbtContext{
			ProjectRoot: testdataRoot,
			ProjectYaml: DbtProjectYaml{
				ProjectName: AnnotatedField[string]{
					Value: "jaffle_shop",
					Position: lsp.Position{
						Line:      0,
						Character: 6,
					},
				},
				Profile: AnnotatedField[string]{
					Value: "jaffle_shop",
					Position: lsp.Position{
						Line:      5,
						Character: 9,
					},
				},
				ModelPaths: AnnotatedField[[]string]{
					Value: []string{"models"},
					Position: lsp.Position{
						Line:      7,
						Character: 13,
					},
				},
				SeedPaths: AnnotatedField[[]string]{
					Value: []string{"seeds"},
					Position: lsp.Position{
						Line:      8,
						Character: 12,
					},
				},
				MacroPaths: AnnotatedField[[]string]{
					Value: []string{"macros"},
					Position: lsp.Position{
						Line:      11,
						Character: 13,
					},
				},
				PackagesInstallPath: AnnotatedField[string]{
					Value: "dbt_packages",
					Position: lsp.Position{
						Line:      0,
						Character: 0,
					},
				},
				DocsPaths: AnnotatedField[[]string]{
					Value: []string{"models", "macros"},
					Position: lsp.Position{
						Line:      0,
						Character: 0,
					},
				},
				Vars: AnnotatedMap{
					"global_count": AnnotatedField[interface{}]{
						Value: 0,
						Position: lsp.Position{
							Line:      36,
							Character: 16,
						},
					},
					"jaffle_shop": AnnotatedField[interface{}]{
						Value: AnnotatedMap{
							"jaffle_number": AnnotatedField[interface{}]{
								Value: 1,
								Position: lsp.Position{
									Line:      40,
									Character: 19,
								},
							},
							"jaffle_string": AnnotatedField[interface{}]{
								Value: "jaffle",
								Position: lsp.Position{
									Line:      39,
									Character: 19,
								},
							},
						},
						Position: lsp.Position{
							Line:      39,
							Character: 4,
						},
					},
				},
			},
			Dialect: docs.Dialect("duckdb"),
			ModelDetailMap: map[string]ModelDetails{
				"chained_ctes": {
					URI:         filepath.Join(testdataRoot, "models/chained_ctes.sql"),
					ProjectName: "jaffle_shop",
					SchemaRange: lsp.Range{},
				},
				"conditional_source": {
					URI:         filepath.Join(testdataRoot, "models/conditional_source.sql"),
					ProjectName: "jaffle_shop",
					SchemaRange: lsp.Range{},
				},
				"customer_orders_summary": {
					URI:         filepath.Join(testdataRoot, "models/customer_orders_summary.sql"),
					ProjectName: "jaffle_shop",
					SchemaRange: lsp.Range{},
				},
				"customers": {
					URI:         filepath.Join(testdataRoot, "models/customers.sql"),
					ProjectName: "jaffle_shop",
					Description: "This table has basic information about a customer, as well as some derived facts based on a customer's orders",
					SchemaURI:   filepath.Join(testdataRoot, "models/schema.yml"),
					SchemaRange: lsp.Range{
						Start: lsp.Position{Line: 3, Character: 10},
						End:   lsp.Position{Line: 3, Character: 10},
					},
					Columns: []Column{
						{Name: "customer_id", Description: "This is a unique identifier for a customer", Position: lsp.Position{Line: 7, Character: 14}},
						{Name: "first_name", Description: "Customer's first name. PII.", Position: lsp.Position{Line: 13, Character: 14}},
						{Name: "last_name", Description: "Customer's last name. PII.", Position: lsp.Position{Line: 16, Character: 14}},
						{Name: "first_order", Description: "Date (UTC) of a customer's first order", Position: lsp.Position{Line: 19, Character: 14}},
						{Name: "most_recent_order", Description: "Date (UTC) of a customer's most recent order", Position: lsp.Position{Line: 22, Character: 14}},
						{Name: "number_of_orders", Description: "Count of the number of orders a customer has placed", Position: lsp.Position{Line: 25, Character: 14}},
						{Name: "total_order_amount", Description: "Total value (AUD) of a customer's orders", Position: lsp.Position{Line: 28, Character: 14}},
					},
				},
				"order_details": {
					URI:         filepath.Join(testdataRoot, "models/order_details.sql"),
					ProjectName: "jaffle_shop",
					SchemaRange: lsp.Range{},
				},
				"orders": {
					URI:         filepath.Join(testdataRoot, "models/orders.sql"),
					ProjectName: "jaffle_shop",
					Description: "This table has basic information about orders, as well as some derived facts based on payments",
					SchemaURI:   filepath.Join(testdataRoot, "models/schema.yml"),
					SchemaRange: lsp.Range{
						Start: lsp.Position{Line: 31, Character: 10},
						End:   lsp.Position{Line: 31, Character: 10},
					},
					Columns: []Column{
						{Name: "order_id", Description: "This is a unique identifier for an order", Position: lsp.Position{Line: 35, Character: 14}},
						{Name: "customer_id", Description: "Foreign key to the customers table", Position: lsp.Position{Line: 41, Character: 14}},
						{Name: "order_date", Description: "Date (UTC) that the order was placed", Position: lsp.Position{Line: 49, Character: 14}},
						{Name: "status", Description: `{{ doc("orders_status") }}`, Position: lsp.Position{Line: 52, Character: 14}},
						{Name: "amount", Description: "Total amount (AUD) of the order", Position: lsp.Position{Line: 58, Character: 14}},
						{Name: "credit_card_amount", Description: "Amount of the order (AUD) paid for by credit card", Position: lsp.Position{Line: 63, Character: 14}},
						{Name: "coupon_amount", Description: "Amount of the order (AUD) paid for by coupon", Position: lsp.Position{Line: 68, Character: 14}},
						{Name: "bank_transfer_amount", Description: "Amount of the order (AUD) paid for by bank transfer", Position: lsp.Position{Line: 73, Character: 14}},
						{Name: "gift_card_amount", Description: "Amount of the order (AUD) paid for by gift card", Position: lsp.Position{Line: 78, Character: 14}},
					},
				},
				"stg_customer_status": {
					URI:         filepath.Join(testdataRoot, "dbt_packages/jaffle_package/models/stg_customer_status.sql"),
					ProjectName: "jaffle_package",
					Description: "",
					SchemaURI:   "",
					SchemaRange: lsp.Range{
						Start: lsp.Position{Line: 0, Character: 0},
						End:   lsp.Position{Line: 0, Character: 0},
					},
				},
				"stg_customers": {
					URI:         filepath.Join(testdataRoot, "models/staging/stg_customers.sql"),
					ProjectName: "jaffle_shop",
					Description: "",
					SchemaURI:   filepath.Join(testdataRoot, "models/staging/schema.yml"),
					SchemaRange: lsp.Range{
						Start: lsp.Position{Line: 3, Character: 10},
						End:   lsp.Position{Line: 3, Character: 10},
					},
					Columns: []Column{
						{Name: "customer_id", Description: "", Position: lsp.Position{Line: 5, Character: 14}},
					},
				},
				"stg_orders": {
					URI:         filepath.Join(testdataRoot, "models/staging/stg_orders.sql"),
					ProjectName: "jaffle_shop",
					Description: "",
					SchemaURI:   filepath.Join(testdataRoot, "models/staging/schema.yml"),
					SchemaRange: lsp.Range{
						Start: lsp.Position{Line: 10, Character: 10},
						End:   lsp.Position{Line: 10, Character: 10},
					},
					Columns: []Column{
						{Name: "order_id", Description: "", Position: lsp.Position{Line: 12, Character: 14}},
						{Name: "status", Description: "", Position: lsp.Position{Line: 16, Character: 14}},
					},
				},
				"stg_payments": {
					URI:         filepath.Join(testdataRoot, "models/staging/stg_payments.sql"),
					ProjectName: "jaffle_shop",
					Description: "",
					SchemaURI:   filepath.Join(testdataRoot, "models/staging/schema.yml"),
					SchemaRange: lsp.Range{
						Start: lsp.Position{Line: 21, Character: 10},
						End:   lsp.Position{Line: 21, Character: 10},
					},
					Columns: []Column{
						{Name: "payment_id", Description: "", Position: lsp.Position{Line: 23, Character: 14}},
						{Name: "payment_method", Description: "", Position: lsp.Position{Line: 27, Character: 14}},
					},
				},
				"raw_customers": {
					URI:         filepath.Join(testdataRoot, "seeds/raw_customers.csv"),
					ProjectName: "jaffle_shop",
					Description: "Seed File",
					SchemaURI:   "",
					SchemaRange: lsp.Range{
						Start: lsp.Position{Line: 0, Character: 0},
						End:   lsp.Position{Line: 0, Character: 0},
					},
				},
				"raw_orders": {
					URI:         filepath.Join(testdataRoot, "seeds/raw_orders.csv"),
					ProjectName: "jaffle_shop",
					Description: "Seed File",
					SchemaURI:   "",
					SchemaRange: lsp.Range{
						Start: lsp.Position{Line: 0, Character: 0},
						End:   lsp.Position{Line: 0, Character: 0},
					},
				},
				"raw_payments": {
					URI:         filepath.Join(testdataRoot, "seeds/raw_payments.csv"),
					ProjectName: "jaffle_shop",
					Description: "Seed File",
					SchemaURI:   "",
					SchemaRange: lsp.Range{
						Start: lsp.Position{Line: 0, Character: 0},
						End:   lsp.Position{Line: 0, Character: 0},
					},
				},
			},
			SourceDetailMap: map[string]Source{
				"jaffle_shop": {
					Name:        "jaffle_shop",
					Description: "",
					URI:         filepath.Join(testdataRoot, "models/schema.yml"),
					Range: lsp.Range{
						Start: lsp.Position{Line: 84, Character: 10},
						End:   lsp.Position{Line: 84, Character: 10},
					},
					Tables: map[string]SourceTable{
						"customers": {
							Name:        "customers",
							Description: "",
							Table:       "jaffle_shop",
							URI:         filepath.Join(testdataRoot, "models/schema.yml"),
							Range: lsp.Range{
								Start: lsp.Position{Line: 96, Character: 14},
								End:   lsp.Position{Line: 96, Character: 14},
							},
							Columns: []Column{
								{Name: "customer_id", Description: "", Position: lsp.Position{Line: 98, Character: 18}},
								{Name: "first_name", Description: "", Position: lsp.Position{Line: 100, Character: 18}},
								{Name: "last_name", Description: "", Position: lsp.Position{Line: 102, Character: 18}},
							},
						},
						"orders": {
							Name:        "orders",
							Description: "",
							Table:       "jaffle_shop",
							URI:         filepath.Join(testdataRoot, "models/schema.yml"),
							Range: lsp.Range{
								Start: lsp.Position{Line: 88, Character: 14},
								End:   lsp.Position{Line: 88, Character: 14},
							},
							Columns: []Column{
								{Name: "order_id", Description: "", Position: lsp.Position{Line: 90, Character: 18}},
								{Name: "customer_id", Description: "", Position: lsp.Position{Line: 92, Character: 18}},
								{Name: "order_date", Description: "", Position: lsp.Position{Line: 94, Character: 18}},
							},
						},
					},
				},
				"stripe": {
					Name:        "stripe",
					Description: "",
					URI:         filepath.Join(testdataRoot, "models/schema.yml"),
					Range: lsp.Range{
						Start: lsp.Position{Line: 105, Character: 10},
						End:   lsp.Position{Line: 105, Character: 10},
					},
					Tables: map[string]SourceTable{
						"payments": {
							Name:        "payments",
							Description: "",
							Table:       "stripe",
							URI:         filepath.Join(testdataRoot, "models/schema.yml"),
							Range: lsp.Range{
								Start: lsp.Position{Line: 107, Character: 14},
								End:   lsp.Position{Line: 107, Character: 14},
							},
							Columns: []Column{
								{Name: "payment_id", Description: "Unique payment identifier", Position: lsp.Position{Line: 109, Character: 18}},
								{Name: "payment_method", Description: "", Position: lsp.Position{Line: 111, Character: 18}},
								{Name: "amount", Description: "", Position: lsp.Position{Line: 113, Character: 18}},
							},
						},
					},
				},
			},
			MacroDetailMap: map[Package]map[string]Macro{
				"jaffle_package": {
					"add_values": {
						Name:        "add_values",
						ProjectName: "jaffle_package",
						Description: "add_values(arg1, arg2)",
						Arguments:   []MacroArg{{Name: "arg1"}, {Name: "arg2"}},
						URI:         filepath.Join(testdataRoot, "dbt_packages/jaffle_package/macros/jaffle_package_macros.sql"),
						Range: lsp.Range{
							Start: lsp.Position{
								Line:      0,
								Character: 9,
							},
							End: lsp.Position{
								Line:      0,
								Character: 31,
							},
						},
					},
				},
				"jaffle_shop": {
					"full_name": {
						Name:        "full_name",
						ProjectName: "jaffle_shop",
						Description: "full_name(first_name, last_name)",
						Arguments:   []MacroArg{{Name: "first_name"}, {Name: "last_name"}},
						URI:         filepath.Join(testdataRoot, "macros/jaffle_macros.sql"),
						Range: lsp.Range{
							Start: lsp.Position{
								Line:      0,
								Character: 9,
							},
							End: lsp.Position{
								Line:      0,
								Character: 41,
							},
						},
					},
					"times_five": {
						Name:        "times_five",
						ProjectName: "jaffle_shop",
						Description: "times_five(int_value)",
						Arguments:   []MacroArg{{Name: "int_value"}},
						URI:         filepath.Join(testdataRoot, "macros/jaffle_macros.sql"),
						Range: lsp.Range{
							Start: lsp.Position{
								Line:      6,
								Character: 10,
							},
							End: lsp.Position{
								Line:      6,
								Character: 31,
							},
						},
					},
				},
			},
			VariableDetailMap: map[string]Variable{
				"global_count": {
					Name:  "global_count",
					Value: 0,
					URI:   filepath.Join(testdataRoot, "dbt_project.yml"),
					Range: lsp.Range{
						Start: lsp.Position{
							Line:      36,
							Character: 16,
						},
						End: lsp.Position{
							Line:      36,
							Character: 16,
						},
					},
				},
				"jaffle_number": {
					Name:  "jaffle_number",
					Value: 1,
					URI:   filepath.Join(testdataRoot, "dbt_project.yml"),
					Range: lsp.Range{
						Start: lsp.Position{
							Line:      40,
							Character: 19,
						},
						End: lsp.Position{
							Line:      40,
							Character: 19,
						},
					},
				},
				"jaffle_string": {
					Name:  "jaffle_string",
					Value: "jaffle",
					URI:   filepath.Join(testdataRoot, "dbt_project.yml"),
					Range: lsp.Range{
						Start: lsp.Position{
							Line:      39,
							Character: 19,
						},
						End: lsp.Position{
							Line:      39,
							Character: 19,
						},
					},
				},
			},
		},
		FusionEnabled: false,
	}

	return expectedState
}

func TestRefreshDbtContext(t *testing.T) {
	testdataRoot, err := testutils.GetTestdataPath("jaffle_shop_duckdb")
	if err != nil {
		t.Fatal(err)
	}

	expectedState := expectedTestState()

	state := NewState()
	state.refreshDbtContext(testdataRoot)

	if !reflect.DeepEqual(state, expectedState) {
		t.Fatalf("expected %#v,\n\ngot %#v", expectedState, state)
	}
}

func BenchmarkRefreshDbtContext(b *testing.B) {
	for i := 0; i < b.N; i++ {
		testdataRoot, err := testutils.GetTestdataPath("jaffle_shop_duckdb")
		if err != nil {
			b.Fatal(err)
		}

		state := NewState()
		state.refreshDbtContext(testdataRoot)
	}
}
