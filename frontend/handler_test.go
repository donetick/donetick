package frontend

import (
	"strings"
	"testing"
)

const sampleIndex = `<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <base href="/" />
    <script type="module" crossorigin src="./assets/index-abc.js"></script>
  </head>
  <body><div id="root"></div></body>
</html>`

func TestRenderIndexAtRootIsUnchanged(t *testing.T) {
	if got := string(renderIndex([]byte(sampleIndex), "")); got != sampleIndex {
		t.Fatalf("index changed at root:\n%s", got)
	}
}

func TestRenderIndexWithPrefix(t *testing.T) {
	got := string(renderIndex([]byte(sampleIndex), "/api/hassio_ingress/tok"))
	for _, want := range []string{
		`<base href="/api/hassio_ingress/tok/" />`,
		`<script>window.__DONETICK_BASE_PATH__="/api/hassio_ingress/tok"</script>`,
		`src="./assets/index-abc.js"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered index missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, `<base href="/" />`) {
		t.Errorf("placeholder base tag was not replaced:\n%s", got)
	}
	if n := strings.Count(got, "<base "); n != 1 {
		t.Errorf("expected exactly one <base> tag, got %d", n)
	}
}

func TestRenderIndexWithoutPlaceholder(t *testing.T) {
	legacy := strings.Replace(sampleIndex, "    <base href=\"/\" />\n", "", 1)
	got := string(renderIndex([]byte(legacy), "/donetick"))
	if !strings.Contains(got, `<head><base href="/donetick/" /><script>window.__DONETICK_BASE_PATH__="/donetick"</script>`) {
		t.Errorf("expected injection right after <head>:\n%s", got)
	}
}
