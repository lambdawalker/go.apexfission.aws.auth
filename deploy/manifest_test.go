package deploy

import (
	"strings"
	"testing"
)

func TestManifestValidation(t *testing.T) {
	valid := `{"version":1,"application":"sales","modules":{"auth":"0.1.0"},"builds":[{"name":"sales","package":"./cmd/sales"}],"environments":{"dev":{"account":"123456789012","region":"us-east-1","backend":"s3://sales-state","stack":"dev","directory":"infra"}}}`
	if _, e := Parse(strings.NewReader(valid)); e != nil {
		t.Fatal(e)
	}
	for _, bad := range []string{strings.Replace(valid, `"version":1`, `"version":2`, 1), strings.Replace(valid, `./cmd/sales`, `../outside`, 1), strings.Replace(valid, `"directory":"infra"`, `"directory":"../outside"`, 1), strings.Replace(valid, `"application":"sales"`, `"application":"sales","unknown":true`, 1)} {
		if _, e := Parse(strings.NewReader(bad)); e == nil {
			t.Fatal("invalid manifest accepted", bad)
		}
	}
}
