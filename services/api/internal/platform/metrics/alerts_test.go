package metrics_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"veyrogram/api/internal/platform/metrics"
)

// Every metric an alert rule refers to must exist in /metrics, otherwise the alert can never fire.
func TestAlertRulesOnlyReferenceExportedMetrics(t *testing.T) {
	raw, err := os.ReadFile("../../../../../infrastructure/prometheus/veyrogram-alerts.yml")
	if err != nil {
		raw, err = os.ReadFile("../../../../infrastructure/prometheus/veyrogram-alerts.yml")
	}
	if err != nil {
		t.Fatal(err)
	}
	r := metrics.NewRegistry()
	metrics.NewPlatform(r)
	db := openFake(t)
	metrics.DBPool(r, db)
	var sb strings.Builder
	r.Write(&sb)
	exported := sb.String()
	names := regexp.MustCompile(`\bvg_[a-z0-9_]+`).FindAllString(string(raw), -1)
	if len(names) == 0 {
		t.Fatal("alert file references no vg_ metrics")
	}
	for _, n := range names {
		base := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(n, "_bucket"), "_sum"), "_count")
		if !strings.Contains(exported, "# TYPE "+base+" ") {
			t.Errorf("alert rule references %s but /metrics does not export %s", n, base)
		}
	}
}
