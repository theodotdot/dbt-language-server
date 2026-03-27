package analysis

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/j-clemons/dbt-language-server/testutils"
)

func TestCreateModelPathMap(t *testing.T) {
	testdataRoot, err := testutils.GetTestdataPath("jaffle_shop_duckdb")
	if err != nil {
		panic(err)
	}
	expectedState := expectedTestState()

	modelPathMap := createModelPathMap(
		expectedState.DbtContext.ProjectRoot,
		expectedState.DbtContext.ProjectYaml,
	)

	expected := map[string]string{
		"chained_ctes":           filepath.Join(testdataRoot, "models/chained_ctes.sql"),
		"conditional_source":     filepath.Join(testdataRoot, "models/conditional_source.sql"),
		"customer_orders_summary": filepath.Join(testdataRoot, "models/customer_orders_summary.sql"),
		"customers":              filepath.Join(testdataRoot, "models/customers.sql"),
		"order_details":          filepath.Join(testdataRoot, "models/order_details.sql"),
		"orders":                 filepath.Join(testdataRoot, "models/orders.sql"),
		"stg_customers":          filepath.Join(testdataRoot, "models/staging/stg_customers.sql"),
		"stg_orders":             filepath.Join(testdataRoot, "models/staging/stg_orders.sql"),
		"stg_payments":           filepath.Join(testdataRoot, "models/staging/stg_payments.sql"),
	}

	if !reflect.DeepEqual(modelPathMap, expected) {
		t.Fatalf("expected %v, got %v", expectedState.DbtContext.ModelDetailMap, modelPathMap)
	}
}
