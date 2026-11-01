package handler

import (
	"net/url"
	"testing"

	"github.com/yumyai/ggtable/pkg/model"
)

func TestParseSort(t *testing.T) {
	tests := []struct {
		query     string
		wantField model.ClusterField
		wantDir   string
	}{
		{"", model.ClusterFieldClusterID, "asc"},
		{"order_by=function&order_dir=desc", model.ClusterFieldFunction, "desc"},
		{"order_by=cog_id&order_dir=DESC", model.ClusterFieldCOGID, "desc"},
		{"order_by=gene_id&order_dir=up", model.ClusterFieldClusterID, "asc"}, // not sortable
		{"order_by=bogus", model.ClusterFieldClusterID, "asc"},
	}
	for _, tt := range tests {
		q, _ := url.ParseQuery(tt.query)
		field, dir := parseSort(q)
		if field != tt.wantField || dir != tt.wantDir {
			t.Errorf("parseSort(%q) = %s %s, want %s %s", tt.query, field, dir, tt.wantField, tt.wantDir)
		}
	}
}
