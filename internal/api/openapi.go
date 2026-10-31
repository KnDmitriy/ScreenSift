package api

import "net/http"

const spec = `openapi: 3.0.3
info:
  title: ScreenSift API
  version: 1.0.0
paths:
  /search:
    get:
      parameters:
      - {name: q, in: query, required: true, schema: {type: string}}
      - {name: limit, in: query, schema: {type: integer, maximum: 100, default: 20}}
      responses: {"200": {description: Search results}}
  /documents:
    get:
      responses: {"200": {description: Documents}}
  /documents/{id}:
    get:
      responses: {"200": {description: Document}}
  /documents/{id}/reindex:
    post:
      responses: {"202": {description: Queued}}
  /stats:
    get:
      responses: {"200": {description: Statistics}}
  /index/rebuild:
    post:
      responses: {"202": {description: Rebuild queued}}
`

func openAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/yaml")
	_, _ = w.Write([]byte(spec))
}
func swagger(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	_, _ = w.Write([]byte(`<!doctype html><html><head><title>ScreenSift Swagger</title><link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist/swagger-ui.css"></head><body><div id="swagger-ui"></div><script src="https://unpkg.com/swagger-ui-dist/swagger-ui-bundle.js"></script><script>SwaggerUIBundle({url:'/openapi.yaml',dom_id:'#swagger-ui'});</script></body></html>`))
}
